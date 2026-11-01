package product

import "github.com/lcfrai/luci/control-plane/internal/management"

// One dedicated project HTTPS endpoint. No arbitrary manifest, shell, resolver,
// DNS server, ACME directory or backend URL crosses this request boundary.
type PublicHTTPSReviewInput struct {
	ReviewKey                        string                   `json:"reviewKey"`
	DeploymentID                     string                   `json:"deploymentId"`
	ServiceID                        string                   `json:"serviceId"`
	Hostname                         string                   `json:"hostname"`
	Path                             string                   `json:"path"`
	PathType                         string                   `json:"pathType"`
	AzureDNSZoneID                   string                   `json:"azureDnsZoneId"`
	TraefikImage                     string                   `json:"traefikImage"`
	StorageClass                     string                   `json:"storageClass"`
	ACMEEmail                        string                   `json:"acmeEmail"`
	DNSCredential                    PublicHTTPSDNSCredential `json:"dnsCredential"`
	MonthlyBudgetUSD                 float64                  `json:"monthlyBudgetUsd"`
	AcknowledgePublicCostsAndRenewal bool                     `json:"acknowledgePublicCostsAndRenewal"`
	AcknowledgeACMETerms             bool                     `json:"acknowledgeAcmeTerms"`
	AcknowledgePrivateDNSCredential  bool                     `json:"acknowledgePrivateDnsCredential"`
}

// Retained only in private project state and a project-owned immutable Secret.
// Never returned in reviews, logs, evidence or the model context.
type PublicHTTPSDNSCredential struct {
	TenantID     string `json:"tenantId"`
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
}
type PublicHTTPSActionInput struct {
	ExpectedRevision int    `json:"expectedRevision"`
	RequestKey       string `json:"requestKey"`
	Action           string `json:"action"` // apply, observe, pause, resume, retire
}
type PublicHTTPSDestination struct {
	TargetID          string `json:"targetId"`
	TargetFingerprint string `json:"targetFingerprint"`
	Namespace         string `json:"namespace"`
	NamespaceUID      string `json:"namespaceUid"`
	ServiceID         string `json:"serviceId"`
	ServiceName       string `json:"serviceName"`
	ServiceUID        string `json:"serviceUid"`
	ServicePort       int    `json:"servicePort"`
}
type PublicHTTPSPlan struct {
	ID                  string                 `json:"id"`
	ProjectID           string                 `json:"projectId"`
	DeploymentID        string                 `json:"deploymentId"`
	Destination         PublicHTTPSDestination `json:"destination"`
	Hostname            string                 `json:"hostname"`
	Path                string                 `json:"path"`
	PathType            string                 `json:"pathType"`
	AzureDNSZoneID      string                 `json:"azureDnsZoneId"`
	AzureDNSZoneETag    string                 `json:"azureDnsZoneEtag"`
	AzureDNSRecordName  string                 `json:"azureDnsRecordName"`
	AzureJobID          string                 `json:"azureJobId"`
	AzureJobPlanDigest  string                 `json:"azureJobPlanDigest"`
	AzureClusterUID     string                 `json:"azureClusterUid"`
	TraefikImage        string                 `json:"traefikImage"`
	Profile             string                 `json:"profile"` // traefik-native-acme-azuredns-file-v1
	StorageClass        string                 `json:"storageClass"`
	StorageClassUID     string                 `json:"storageClassUid"`
	StorageClassVersion string                 `json:"storageClassVersion"`
	StorageBindingMode  string                 `json:"storageBindingMode"`
	StorageGiB          int64                  `json:"storageGiB"`
	ACMEEmail           string                 `json:"acmeEmail"`
	DNSClientID         string                 `json:"dnsClientId"`
	MonthlyBudgetUSD    float64                `json:"monthlyBudgetUsd"`
	CreatedAt           string                 `json:"createdAt"`
	ExpiresAt           string                 `json:"expiresAt"`
	Digest              string                 `json:"digest"`
}
type PublicHTTPSResource struct {
	Kind            string `json:"kind"`
	Name            string `json:"name"`
	Attempted       bool   `json:"attempted"`
	UID             string `json:"uid,omitempty"`
	State           string `json:"state"`
	DeleteAttempted bool   `json:"deleteAttempted"`
	AbsentObserved  bool   `json:"absentObserved"`
}
type PublicHTTPSActionReceipt struct {
	RequestKey      string `json:"requestKey"`
	RequestDigest   string `json:"requestDigest"`
	Action          string `json:"action"`
	State           string `json:"state"` // attempted, settled, unknown
	StartedRevision int    `json:"startedRevision"`
	SettledRevision int    `json:"settledRevision"`
}
type PublicHTTPSStatus struct {
	ID                        string                     `json:"id"`
	ProjectID                 string                     `json:"projectId"`
	Revision                  int                        `json:"revision"`
	ReviewKey                 string                     `json:"reviewKey"`
	LastActionRequestKey      string                     `json:"lastActionRequestKey"`
	Actions                   []PublicHTTPSActionReceipt `json:"actions"`
	Plan                      PublicHTTPSPlan            `json:"plan"`
	State                     string                     `json:"state"` // reviewed, provisioning, pending-address, pending-dns, pending-certificate, ready, paused, retiring, retired, unknown, access-changed
	Resources                 []PublicHTTPSResource      `json:"resources"`
	Address                   string                     `json:"address"`
	DNSAttempted              bool                       `json:"dnsAttempted"`
	DNSAcknowledged           bool                       `json:"dnsAcknowledged"`
	DNSObserved               bool                       `json:"dnsObserved"`
	HTTPSVerified             bool                       `json:"httpsVerified"`
	CertificateNotAfter       string                     `json:"certificateNotAfter,omitempty"`
	CertificateSHA256         string                     `json:"certificateSha256,omitempty"`
	HTTPStatus                int                        `json:"httpStatus"`
	ApplicationHealthVerified bool                       `json:"applicationHealthVerified"`
	URL                       string                     `json:"url,omitempty"`
	ObservedAt                string                     `json:"observedAt"`
	LastAction                string                     `json:"lastAction"`
	Message                   string                     `json:"message"`
	PVCDataRetained           bool                       `json:"pvcDataRetained"`
}
type storedPublicHTTPS struct {
	Version      int                          `json:"version"`
	AccountScope string                       `json:"accountScope"`
	ProjectScope string                       `json:"projectScope"`
	Actor        management.ExecutionActorRef `json:"actor"`
	PublicHTTPSStatus
	DNSCredential         PublicHTTPSDNSCredential `json:"dnsCredential"`
	SourceBlueprintDigest string                   `json:"sourceBlueprintDigest"`
	SourceContextDigest   string                   `json:"sourceContextDigest"`
	SourceRunDigest       string                   `json:"sourceRunDigest"`
	SourceCheckDigest     string                   `json:"sourceCheckDigest"`
	RequestKey            string                   `json:"requestKey"`
	RequestDigest         string                   `json:"requestDigest"`
	ReviewRequestDigest   string                   `json:"reviewRequestDigest"`
	PauseAttempted        bool                     `json:"pauseAttempted"`
	ResumeAttempted       bool                     `json:"resumeAttempted"`
	ScaleVersion          string                   `json:"scaleVersion"`
	DesiredReplicas       int32                    `json:"desiredReplicas"`
	ApplyAttempted        bool                     `json:"applyAttempted"`
	RetireAttempted       bool                     `json:"retireAttempted"`
	DNSRecordETag         string                   `json:"dnsRecordEtag"`
	DNSDeleteAttempted    bool                     `json:"dnsDeleteAttempted"`
	DNSAbsentObserved     bool                     `json:"dnsAbsentObserved"`
	NextObserveAt         string                   `json:"nextObserveAt"`
}
