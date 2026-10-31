package product

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/deploymentplan"
)

func runtimeCertificateSerial() (*big.Int, error) {
	n, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, err
	}
	n.SetBit(n, 127, 1)
	return n, nil
}
func runtimePrivatePEM(key *ecdsa.PrivateKey) ([]byte, error) {
	raw, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: raw}), nil
}
func runtimeTLSSecrets(blueprint deploymentplan.Blueprint) ([]RuntimeSecretPlan, []runtimePrivateSecret, error) {
	plans := []RuntimeSecretPlan{}
	private := []runtimePrivateSecret{}
	names := map[string]bool{}
	for _, collector := range blueprint.Services {
		if collector.Collector == nil {
			continue
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
		ca := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: "Luci internal telemetry CA"}, NotBefore: now.Add(-5 * time.Minute), NotAfter: expiry, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign, BasicConstraintsValid: true, IsCA: true, MaxPathLenZero: true}
		caDER, err := x509.CreateCertificate(rand.Reader, ca, ca, &caKey.PublicKey, caKey)
		if err != nil {
			return nil, nil, err
		}
		caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})
		issue := func(serviceID, kind string, ref deploymentplan.TelemetryTLSReference, dns []string) error {
			if names[ref.ResourceName] || ref.ResourceName == "" {
				return ErrRuntimePreparation
			}
			names[ref.ResourceName] = true
			key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
			if err != nil {
				return err
			}
			serial, err := runtimeCertificateSerial()
			if err != nil {
				return err
			}
			usage := []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
			if kind == "telemetry-server" {
				usage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
			}
			cert := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: serviceID}, DNSNames: dns, NotBefore: now.Add(-5 * time.Minute), NotAfter: expiry, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: usage, BasicConstraintsValid: true}
			der, err := x509.CreateCertificate(rand.Reader, cert, ca, &key.PublicKey, caKey)
			if err != nil {
				return err
			}
			keyPEM, err := runtimePrivatePEM(key)
			if err != nil {
				return err
			}
			generation, err := kubeOpaqueID("secret-")
			if err != nil {
				return err
			}
			plans = append(plans, RuntimeSecretPlan{Name: ref.ResourceName, Kind: kind, Keys: []string{ref.CAKey, ref.CertKey, ref.KeyKey}, GenerationID: generation, ExpiresAt: expiry.Format(time.RFC3339Nano), ServiceID: serviceID})
			private = append(private, runtimePrivateSecret{Name: ref.ResourceName, Type: "Opaque", GenerationID: generation, Data: map[string][]byte{ref.CAKey: caPEM, ref.CertKey: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), ref.KeyKey: keyPEM}})
			return nil
		}
		name := deploymentplan.ResourceName(blueprint.ProjectID, collector.ID)
		dns := []string{name, name + "." + blueprint.Infrastructure.Namespace, name + "." + blueprint.Infrastructure.Namespace + ".svc", name + "." + blueprint.Infrastructure.Namespace + ".svc.cluster.local"}
		if err = issue(collector.ID, "telemetry-server", collector.Collector.ReceiverTLS, dns); err != nil {
			return nil, nil, err
		}
		for _, app := range blueprint.Services {
			if app.Telemetry == nil || app.Telemetry.ResourceID != collector.ID {
				continue
			}
			if app.Telemetry.ClientTLS == nil {
				return nil, nil, ErrRuntimePreparation
			}
			if err = issue(app.ID, "telemetry-client", *app.Telemetry.ClientTLS, nil); err != nil {
				return nil, nil, err
			}
		}
		// The CA signing key is not sent to Kubernetes and is not retained in a task,
		// review, browser response or evidence. Exact issued bundles are privately
		// persisted before their first Secret Create; retry never regenerates them.
	}
	storagePlans, storagePrivate, err := runtimeStorageTLSSecrets(blueprint)
	if err != nil {
		return nil, nil, err
	}
	for _, p := range storagePlans {
		if names[p.Name] {
			return nil, nil, ErrRuntimePreparation
		}
		names[p.Name] = true
	}
	return append(plans, storagePlans...), append(private, storagePrivate...), nil
}
