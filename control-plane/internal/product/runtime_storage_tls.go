package product

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"github.com/lcfrai/luci/control-plane/internal/deploymentplan"
	"time"
)

// Reuses the existing resource preparation's private synced Secret bytes,
// exclusive create/owned UID/observe/no-replay lifecycle. It creates no CA
// resource or long-lived signer and never puts PEM values in a public plan.
func runtimeStorageTLSSecrets(b deploymentplan.Blueprint) ([]RuntimeSecretPlan, []runtimePrivateSecret, error) {
	plans := []RuntimeSecretPlan{}
	private := []runtimePrivateSecret{}
	needed := false
	for _, s := range b.Services {
		needed = needed || s.TelemetryStorage != nil
	}
	if !needed {
		return plans, private, nil
	}
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	serial, err := runtimeCertificateSerial()
	if err != nil {
		return nil, nil, err
	}
	now := time.Now().UTC()
	expiry := now.Add(90 * 24 * time.Hour)
	ca := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: "Luci owned telemetry storage CA"}, NotBefore: now.Add(-5 * time.Minute), NotAfter: expiry, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign, BasicConstraintsValid: true, IsCA: true, MaxPathLenZero: true}
	der, err := x509.CreateCertificate(rand.Reader, ca, ca, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, nil, err
	}
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	names := map[string]bool{}
	issue := func(id, kind string, ref deploymentplan.TelemetryTLSReference, dns []string) error {
		if ref.ResourceName == "" || ref.CAKey == "" || ref.CertKey == "" || ref.KeyKey == "" || ref.CAKey == ref.CertKey || ref.CAKey == ref.KeyKey || ref.CertKey == ref.KeyKey || names[ref.ResourceName] {
			return ErrRuntimePreparation
		}
		names[ref.ResourceName] = true
		key, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if e != nil {
			return e
		}
		number, e := runtimeCertificateSerial()
		if e != nil {
			return e
		}
		usage := []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
		if kind == "telemetry-server" {
			usage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
		}
		leaf := &x509.Certificate{SerialNumber: number, Subject: pkix.Name{CommonName: id}, DNSNames: dns, NotBefore: now.Add(-5 * time.Minute), NotAfter: expiry, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: usage, BasicConstraintsValid: true}
		bytes, e := x509.CreateCertificate(rand.Reader, leaf, ca, &key.PublicKey, caKey)
		if e != nil {
			return e
		}
		privatePEM, e := runtimePrivatePEM(key)
		if e != nil {
			return e
		}
		generation, e := kubeOpaqueID("secret-")
		if e != nil {
			return e
		}
		plans = append(plans, RuntimeSecretPlan{Name: ref.ResourceName, Kind: kind, Keys: []string{ref.CAKey, ref.CertKey, ref.KeyKey}, GenerationID: generation, ExpiresAt: expiry.Format(time.RFC3339Nano), ServiceID: id})
		private = append(private, runtimePrivateSecret{Name: ref.ResourceName, Type: "Opaque", GenerationID: generation, Data: map[string][]byte{ref.CAKey: caPEM, ref.CertKey: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: bytes}), ref.KeyKey: privatePEM}})
		return nil
	}
	for _, s := range b.Services {
		if v := s.TelemetryStorage; v != nil {
			name := deploymentplan.ResourceName(b.ProjectID, s.ID)
			dns := []string{name, name + "." + b.Infrastructure.Namespace, name + "." + b.Infrastructure.Namespace + ".svc", name + "." + b.Infrastructure.Namespace + ".svc.cluster.local"}
			if err = issue(s.ID, "telemetry-server", v.ServerTLS, dns); err != nil {
				return nil, nil, err
			}
			if err = issue(s.ID, "telemetry-client", v.QueryTLS, nil); err != nil {
				return nil, nil, err
			}
		}
		if s.Collector != nil && s.Collector.OwnedDestination != nil {
			if err = issue(s.ID, "telemetry-client", s.Collector.OwnedDestination.ClientTLS, nil); err != nil {
				return nil, nil, err
			}
		}
	}
	return plans, private, nil
}
