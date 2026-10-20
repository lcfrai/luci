// Exact pure wire mirror; inert source-regression compares these declarations
// and validation with backend/document_image_assembly.go. No runtime dependencies.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
	"time"
)

const (
	DocumentImageAssemblyProtocol         = 1
	DocumentImageAssemblyProfile          = "document-flat-oci-v1"
	DocumentImageAssemblyBaseID           = "sha256:db9f02c6bde9fa90cc8074c92754b2b046947392f1a857726e77f041febb7b82"
	DocumentImageAssemblyInputMaxBytes    = 256 << 20
	DocumentImageAssemblyManifestMaxBytes = 256 << 10
	DocumentImageAssemblyHelperMaxBytes   = 32 << 20
	DocumentImageAssemblyOutputMaxBytes   = 2 << 30
	DocumentImageAssemblyRawMaxBytes      = DocumentImageAssemblyOutputMaxBytes + (16 << 20)
	DocumentImageAssemblyComputeBudget    = 300 * time.Second
	DocumentImageAssemblyCleanupBudget    = 120 * time.Second
	DocumentImageAssemblyWholeBudget      = DocumentImageAssemblyComputeBudget + DocumentImageAssemblyCleanupBudget
)

// This fixed recipe is a distinct identity, never a retry/adoption of a runtime
// producer, CheckExecutor, parser, agent, public route or arbitrary Dockerfile.
const DocumentImageAssemblyRecipe = "document-flat-oci-v1\nclosed-inputs;chroot-dpkg-unpack-configure;pip-no-index-no-deps-only-binary-hashes;eng-osd;verify-installed;flat-oci\nuid=0;caps=CHOWN,DAC_OVERRIDE,FOWNER,FSETID,SETUID,SETGID,SYS_CHROOT,MKNOD;memory=2147483648;swap-total=2147483648;cpu=1000;pids=64;work=2147483648;tmp=134217728;network=none;root=readonly;no-sys-admin;no-proc-mount\n"

type DocumentImageAssemblyInput struct {
	Path   string `json:"path"`
	Bytes  int64  `json:"bytes"`
	Digest string `json:"digest"`
}
type DocumentImageAssemblyPackage struct {
	DocumentImageAssemblyInput
	Name         string `json:"name"`
	Version      string `json:"version"`
	Architecture string `json:"architecture"`
}
type DocumentImageAssemblyManifest struct {
	Version                   int                            `json:"version"`
	Profile                   string                         `json:"profile"`
	SBOMClosed                bool                           `json:"sbomClosed"`
	Authenticated             bool                           `json:"authenticated"`
	DistributionNoticesClosed bool                           `json:"distributionNoticesClosed"`
	SourceCommit              string                         `json:"sourceCommit"`
	SourceDigest              string                         `json:"sourceDigest"`
	HelperDigest              string                         `json:"helperDigest"`
	RecipeDigest              string                         `json:"recipeDigest"`
	SourceRetentionDigest     string                         `json:"sourceRetentionDigest"`
	BaseLayer                 DocumentImageAssemblyInput     `json:"baseLayer"`
	Packages                  []DocumentImageAssemblyPackage `json:"packages"`
	Wheels                    []DocumentImageAssemblyInput   `json:"wheels"`
	Extractor                 DocumentImageAssemblyInput     `json:"extractor"`
	Verifier                  DocumentImageAssemblyInput     `json:"verifier"`
	Requirements              DocumentImageAssemblyInput     `json:"requirements"`
	DependencyLock            DocumentImageAssemblyInput     `json:"dependencyLock"`
	Notices                   []DocumentImageAssemblyInput   `json:"notices"`
	VerificationInputs        []DocumentImageAssemblyInput   `json:"verificationInputs,omitempty"`
}

var assemblyCommit = regexp.MustCompile(`^[0-9a-f]{40}$`)
var assemblyPackage = regexp.MustCompile(`^[a-z0-9][a-z0-9+.-]{0,199}$`)
var assemblyHex = regexp.MustCompile(`^[0-9a-f]{64}$`)

func documentHex(value string) bool { return assemblyHex.MatchString(value) }

func DocumentImageAssemblyRecipeDigest() string {
	sum := sha256.Sum256([]byte(DocumentImageAssemblyRecipe))
	return hex.EncodeToString(sum[:])
}
func ValidateDocumentImageAssemblyManifest(value DocumentImageAssemblyManifest) bool {
	if value.Version != 1 || value.Profile != DocumentImageAssemblyProfile || !value.SBOMClosed || !value.Authenticated || !value.DistributionNoticesClosed || !assemblyCommit.MatchString(value.SourceCommit) || !documentHex(value.SourceDigest) || !documentHex(value.HelperDigest) || !documentHex(value.SourceRetentionDigest) || value.RecipeDigest != DocumentImageAssemblyRecipeDigest() || len(value.Packages) != 174 || len(value.Wheels) != 2 || len(value.Notices) < 1 || len(value.Notices) > 512 {
		return false
	}
	if value.Extractor.Path != "document_extractor.py" || value.Verifier.Path != "verify_image.py" || value.Requirements.Path != "wheel-requirements.txt" || value.DependencyLock.Path != "dependency-lock.json" || value.BaseLayer.Digest != "774043ccc8ccd0d0833a9ee0792142ab7ad93df971e59dd248fbf82db16d0150" || value.BaseLayer.Bytes != 28238443 {
		return false
	}
	inputs := []DocumentImageAssemblyInput{value.BaseLayer, value.Extractor, value.Verifier, value.Requirements, value.DependencyLock}
	inputs = append(inputs, value.Wheels...)
	inputs = append(inputs, value.Notices...)
	inputs = append(inputs, value.VerificationInputs...)
	packages := map[string]bool{}
	for _, item := range value.Packages {
		if !assemblyPackage.MatchString(item.Name) || packages[item.Name] || len(item.Version) < 1 || len(item.Version) > 256 || strings.ContainsAny(item.Version, "\x00\r\n") || item.Architecture != "amd64" && item.Architecture != "all" || !strings.HasPrefix(item.Path, "debs/") || !strings.HasSuffix(item.Path, ".deb") {
			return false
		}
		packages[item.Name] = true
		inputs = append(inputs, item.DocumentImageAssemblyInput)
	}
	wheels := map[string]bool{}
	for _, item := range value.Wheels {
		if !strings.HasPrefix(item.Path, "wheels/") || !strings.HasSuffix(item.Path, ".whl") {
			return false
		}
		switch item.Digest {
		case "11f281613fa22313d9c7ab89947665e84eccf8ebe40e1198a84a88352305648d":
			if item.Bytes != 3799581 {
				return false
			}
		case "23d27a3e0307ec2244cc51e7287b919aa68d097504ebe19df4e76a98a3eea5bd":
			if item.Bytes != 6934408 {
				return false
			}
		default:
			return false
		}
		if wheels[item.Digest] {
			return false
		}
		wheels[item.Digest] = true
	}
	seen, total := map[string]bool{}, int64(0)
	for _, item := range inputs {
		if item.Bytes <= 0 || item.Bytes > DocumentImageAssemblyInputMaxBytes-total || !documentHex(item.Digest) || len(item.Path) > 1024 || item.Path == "" || strings.ContainsAny(item.Path, "\\\x00\r\n:") || strings.HasPrefix(item.Path, "/") || seen[strings.ToLower(item.Path)] {
			return false
		}
		for _, part := range strings.Split(item.Path, "/") {
			if part == "" || part == "." || part == ".." {
				return false
			}
		}
		seen[strings.ToLower(item.Path)], total = true, total+item.Bytes
	}
	for _, item := range value.Notices {
		if !strings.HasPrefix(item.Path, "notices/") {
			return false
		}
	}
	if len(value.VerificationInputs)>64 { return false }
	verificationBytes:=int64(0)
	for i,item:=range value.VerificationInputs {
		if item.Bytes>1<<20 || !regexp.MustCompile(`^verification/installed-[0-9]{4}\.json$`).MatchString(item.Path) || i>0&&value.VerificationInputs[i-1].Path>=item.Path { return false }
		verificationBytes+=item.Bytes
		if verificationBytes>32<<20 { return false }
	}
	return true
}

func DocumentImageAssemblyInputs(value DocumentImageAssemblyManifest) []DocumentImageAssemblyInput {
	items := []DocumentImageAssemblyInput{value.BaseLayer, value.Extractor, value.Verifier, value.Requirements, value.DependencyLock}
	for _, item := range value.Packages {
		items = append(items, item.DocumentImageAssemblyInput)
	}
	items = append(items, value.Wheels...)
	items=append(items, value.Notices...)
	return append(items,value.VerificationInputs...)
}
