package product

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
)

var slugPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{1,39}$`)
var regionPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{1,39}$`)
var domainPattern = regexp.MustCompile(`^(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,63}$`)

func validDomain(v string) bool { return len(v) <= 253 && domainPattern.MatchString(v) }

type CloudRequest struct {
	Provider         string  `json:"provider"`
	Name             string  `json:"name"`
	Region           string  `json:"region"`
	Domain           string  `json:"domain"`
	Size             string  `json:"size"`
	MonthlyBudgetUSD float64 `json:"monthlyBudgetUsd"`
	SSHPublicKey     string  `json:"sshPublicKey"`
}
type Estimate struct {
	Currency    string   `json:"currency"`
	Monthly     *float64 `json:"monthly"`
	Verified    bool     `json:"verified"`
	Explanation string   `json:"explanation"`
}
type PlanStep struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
}
type PlanFile struct {
	Name      string `json:"name"`
	Content   string `json:"content"`
	MediaType string `json:"mediaType"`
}
type PlanCommand struct {
	Label      string   `json:"label"`
	Executable string   `json:"executable"`
	Args       []string `json:"args"`
}
type CloudPlan struct {
	MonthlyBudgetUSD float64       `json:"monthlyBudgetUsd"`
	ID               string        `json:"id"`
	Provider         string        `json:"provider"`
	Name             string        `json:"name"`
	Region           string        `json:"region"`
	Domain           string        `json:"domain"`
	Resources        []string      `json:"resources"`
	Estimate         Estimate      `json:"estimate"`
	Steps            []PlanStep    `json:"steps"`
	Files            []PlanFile    `json:"files"`
	Commands         []PlanCommand `json:"commands"`
	Warnings         []string      `json:"warnings"`
	CreatedAt        time.Time     `json:"createdAt"`
}

func validateCloud(r CloudRequest) error {
	if r.Provider != "aws" && r.Provider != "azure" {
		return errors.New("choose AWS or Azure")
	}
	if !slugPattern.MatchString(r.Name) {
		return errors.New("name must start with a letter and contain 2–40 lowercase letters, digits or hyphens")
	}
	if !regionPattern.MatchString(r.Region) {
		return errors.New("enter a valid provider region")
	}
	if !validDomain(r.Domain) {
		return errors.New("enter a domain you control, without https:// or a path")
	}
	if r.Size != "small" && r.Size != "standard" {
		return errors.New("choose small or standard size")
	}
	if r.MonthlyBudgetUSD <= 0 || r.MonthlyBudgetUSD > 100000 {
		return errors.New("enter your monthly cloud budget in USD")
	}
	parts := strings.Fields(r.SSHPublicKey)
	if len(parts) < 2 || len(parts) > 3 || !(parts[0] == "ssh-ed25519" || parts[0] == "ssh-rsa" || parts[0] == "ecdsa-sha2-nistp256") {
		return errors.New("paste a public SSH key; Luci never needs it in a template as a private key")
	}
	if data, err := base64.StdEncoding.DecodeString(parts[1]); err != nil || len(data) < 32 || len(data) > 16384 {
		return errors.New("SSH public key encoding is invalid")
	}
	return nil
}

func (s *Server) handleCloudPlan(w http.ResponseWriter, r *http.Request) {
	var req CloudRequest
	if !readJSON(w, r, &req) {
		return
	}
	req.Domain = strings.ToLower(strings.TrimSpace(req.Domain))
	req.SSHPublicKey = strings.TrimSpace(req.SSHPublicKey)
	if err := validateCloud(req); err != nil {
		problem(w, 400, "bad_request", err.Error())
		return
	}
	plan, err := s.buildCloudPlan(r.Context(), req)
	if err != nil {
		problem(w, 500, "plan_failed", err.Error())
		return
	}
	s.mu.Lock()
	s.plans[plan.ID] = plan
	s.mu.Unlock()
	writeJSON(w, 200, plan)
}

func (s *Server) buildCloudPlan(ctx context.Context, r CloudRequest) (CloudPlan, error) {
	p := CloudPlan{ID: newID("cp-"), MonthlyBudgetUSD: r.MonthlyBudgetUSD, Provider: r.Provider, Name: r.Name, Region: r.Region, Domain: r.Domain, CreatedAt: time.Now().UTC(), Warnings: []string{},
		Resources: []string{"One Linux virtual machine", "Persistent storage", "A public address", "HTTPS for your domain"},
		Estimate:  Estimate{Currency: "USD", Explanation: "No verified quote is available. Check the provider estimate before creating resources."},
		Steps: []PlanStep{
			{ID: "account", Title: "Connect your account", Detail: "Use the provider's own sign-in. Credentials stay with its local credential store."},
			{ID: "quote", Title: "Review resources and cost", Detail: "Choose one machine. Storage, traffic, public addresses, tax and model usage can add cost."},
			{ID: "preview", Title: "Preview the changes", Detail: "Validate this complete template before the create step."},
			{ID: "create", Title: "Create your Luci machine", Detail: "An explicit cost acknowledgement is required before provisioning."},
			{ID: "dns", Title: "Connect your domain", Detail: "Add an A record for " + r.Domain + " pointing at the new public address. Luci checks DNS before verifying HTTPS."},
			{ID: "connect", Title: "Connect securely", Detail: "Luci's control service stays on loopback. Connect it over an SSH tunnel with your key; the public domain is for shipped apps."},
			{ID: "cleanup", Title: "Keep control of costs", Detail: "Export your projects before deleting the named resources. Stopping Luci or the machine does not guarantee billing stops; confirm removal in your provider account."},
		}}
	bootstrap := bootstrapScript(r)
	p.Files = []PlanFile{{Name: "bootstrap.sh", Content: bootstrap, MediaType: "text/x-shellscript"}}
	if r.Provider == "aws" {
		bundle := "medium_3_0"
		if r.Size == "standard" {
			bundle = "large_3_0"
		}
		// A verified quote must come from the user's selected region and account.
		queryCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
		out, err := s.runner(queryCtx, "aws", []string{"lightsail", "get-bundles", "--region", r.Region, "--output", "json"}, "")
		cancel()
		var quotes struct {
			Bundles []struct {
				BundleID string  `json:"bundleId"`
				Price    float64 `json:"price"`
				IsActive bool    `json:"isActive"`
			} `json:"bundles"`
		}
		if err == nil && json.Unmarshal([]byte(out), &quotes) == nil {
			for _, q := range quotes.Bundles {
				if q.BundleID == bundle && q.IsActive {
					value := q.Price
					p.Estimate.Monthly = &value
					p.Estimate.Verified = true
					p.Estimate.Explanation = "AWS Lightsail bundle quote from your selected region; excess traffic, snapshots, tax and AI usage are additional."
				}
			}
		}
		template := map[string]any{"AWSTemplateFormatVersion": "2010-09-09", "Description": "Luci personal cloud machine; no Kubernetes or managed database", "Resources": map[string]any{
			"Machine": map[string]any{"Type": "AWS::Lightsail::Instance", "Properties": map[string]any{"InstanceName": r.Name, "BlueprintId": "ubuntu_24_04", "BundleId": bundle, "UserData": bootstrap, "Networking": map[string]any{"Ports": []any{
				map[string]any{"FromPort": 22, "ToPort": 22, "Protocol": "tcp", "Cidrs": []string{"0.0.0.0/0"}},
				map[string]any{"FromPort": 80, "ToPort": 80, "Protocol": "tcp", "Cidrs": []string{"0.0.0.0/0"}},
				map[string]any{"FromPort": 443, "ToPort": 443, "Protocol": "tcp", "Cidrs": []string{"0.0.0.0/0"}},
			}}, "Tags": []any{map[string]string{"Key": "managed-by", "Value": "luci"}}}},
			"Address": map[string]any{"Type": "AWS::Lightsail::StaticIp", "Properties": map[string]any{"StaticIpName": r.Name + "-address", "AttachedTo": map[string]string{"Ref": "Machine"}}},
		}, "Outputs": map[string]any{"PublicIp": map[string]any{"Value": map[string]any{"Fn::GetAtt": []string{"Address", "IpAddress"}}}, "Domain": map[string]string{"Value": r.Domain}}}
		data, err := json.MarshalIndent(template, "", "  ")
		if err != nil {
			return p, err
		}
		p.Files = append(p.Files, PlanFile{Name: "template.json", Content: string(data), MediaType: "application/json"})
		p.Commands = []PlanCommand{
			{Label: "Validate", Executable: "aws", Args: []string{"cloudformation", "validate-template", "--template-body", "file://template.json", "--region", r.Region}},
			{Label: "Create", Executable: "aws", Args: []string{"cloudformation", "create-stack", "--stack-name", r.Name, "--template-body", "file://template.json", "--region", r.Region}},
			{Label: "Check progress", Executable: "aws", Args: []string{"cloudformation", "describe-stacks", "--stack-name", r.Name, "--region", r.Region}},
			{Label: "Delete resources", Executable: "aws", Args: []string{"cloudformation", "delete-stack", "--stack-name", r.Name, "--region", r.Region}},
		}
	} else {
		vmSize := "Standard_B2s"
		if r.Size == "standard" {
			vmSize = "Standard_B2ms"
		}
		template := azureTemplate(r, vmSize, bootstrap)
		data, err := json.MarshalIndent(template, "", "  ")
		if err != nil {
			return p, err
		}
		p.Files = append(p.Files, PlanFile{Name: "template.json", Content: string(data), MediaType: "application/json"})
		p.Commands = []PlanCommand{
			{Label: "Create isolated resource group", Executable: "az", Args: []string{"group", "create", "--name", r.Name, "--location", r.Region}},
			{Label: "Preview", Executable: "az", Args: []string{"deployment", "group", "what-if", "--resource-group", r.Name, "--template-file", "template.json"}},
			{Label: "Create", Executable: "az", Args: []string{"deployment", "group", "create", "--resource-group", r.Name, "--name", r.Name, "--template-file", "template.json"}},
			{Label: "Check progress", Executable: "az", Args: []string{"deployment", "group", "show", "--resource-group", r.Name, "--name", r.Name}},
			{Label: "Delete resources", Executable: "az", Args: []string{"group", "delete", "--name", r.Name, "--yes"}},
		}
		p.Estimate.Explanation = "Azure VM, disk and public-address prices vary by region. Obtain the Azure quote before approving this plan."
	}
	if p.Estimate.Monthly != nil && *p.Estimate.Monthly > r.MonthlyBudgetUSD {
		p.Warnings = append(p.Warnings, "The machine quote exceeds your monthly budget. Choose a smaller configuration or increase the budget explicitly.")
	}
	if !p.Estimate.Verified {
		p.Warnings = append(p.Warnings, "This plan does not yet have a verified complete price. Provisioning must acknowledge that uncertainty.")
	}
	p.Warnings = append(p.Warnings, "The server installs Coolify 4.3.23 for application deployment. Open its first-account setup only through Luci's SSH connection; management port 8000 is not opened in the cloud firewall.", "Container isolation shares the host kernel. This personal single-machine configuration is not hostile multi-tenant isolation.")
	return p, nil
}

func bootstrapScript(r CloudRequest) string {
	// All interpolated values passed strict domain/slug/public-key validation.
	key := strings.Join(strings.Fields(r.SSHPublicKey)[:2], " ")
	return fmt.Sprintf(`#!/bin/bash
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get install -y docker.io ca-certificates curl tar openssl git sudo
systemctl enable --now docker
id luci >/dev/null 2>&1 || useradd --create-home --shell /bin/bash luci
usermod -aG docker,sudo luci
printf 'luci ALL=(ALL) NOPASSWD: ALL\n' > /etc/sudoers.d/luci
chmod 440 /etc/sudoers.d/luci
install -d -m 700 -o luci -g luci /home/luci/.ssh /home/luci/.local/state/luci
printf '%%s\n' '%s' > /home/luci/.ssh/authorized_keys
chmod 600 /home/luci/.ssh/authorized_keys
chown luci:luci /home/luci/.ssh/authorized_keys
install -d -m 700 /opt/luci
curl --fail --proto '=https' --tlsv1.2 --location https://raw.githubusercontent.com/coollabsio/coolify/e2e2d4010bcd590084b66d6f748f3eec8e2bbee9/scripts/install.sh --output /opt/luci/coolify-install.sh
printf '%%s  %%s\n' '8ef02dce49339208f5abc247bff0277c73d04538d7a36dcfd21331e314e0f2cd' '/opt/luci/coolify-install.sh' | sha256sum --check --status
AUTOUPDATE=false bash /opt/luci/coolify-install.sh 4.3.23
printf 'Luci deployment machine ready. Open management through SSH. Domain: %s\n'
`, key, r.Domain)
}

func azureTemplate(r CloudRequest, size, bootstrap string) map[string]any {
	resource := func(kind, version, name string, properties map[string]any, depends []string) map[string]any {
		out := map[string]any{"type": kind, "apiVersion": version, "name": name, "location": r.Region, "properties": properties, "tags": map[string]string{"managed-by": "luci"}}
		if len(depends) > 0 {
			out["dependsOn"] = depends
		}
		return out
	}
	vnet := r.Name + "-network"
	nsg := r.Name + "-firewall"
	ip := r.Name + "-address"
	nic := r.Name + "-nic"
	rid := func(kind, name string) string { return fmt.Sprintf("[resourceId('%s', '%s')]", kind, name) }
	rules := []any{}
	for i, port := range []int{22, 80, 443} {
		rules = append(rules, map[string]any{"name": fmt.Sprintf("port-%d", port), "properties": map[string]any{"priority": 100 + i, "protocol": "Tcp", "access": "Allow", "direction": "Inbound", "sourcePortRange": "*", "destinationPortRange": fmt.Sprint(port), "sourceAddressPrefix": "*", "destinationAddressPrefix": "*"}})
	}
	resources := []any{
		resource("Microsoft.Network/networkSecurityGroups", "2023-11-01", nsg, map[string]any{"securityRules": rules}, nil),
		resource("Microsoft.Network/virtualNetworks", "2023-11-01", vnet, map[string]any{"addressSpace": map[string]any{"addressPrefixes": []string{"10.42.0.0/16"}}, "subnets": []any{map[string]any{"name": "luci", "properties": map[string]string{"addressPrefix": "10.42.0.0/24"}}}}, nil),
	}
	ipResource := resource("Microsoft.Network/publicIPAddresses", "2023-11-01", ip, map[string]any{"publicIPAllocationMethod": "Static"}, nil)
	ipResource["sku"] = map[string]string{"name": "Standard"}
	resources = append(resources, ipResource)
	resources = append(resources, resource("Microsoft.Network/networkInterfaces", "2023-11-01", nic, map[string]any{"networkSecurityGroup": map[string]string{"id": rid("Microsoft.Network/networkSecurityGroups", nsg)}, "ipConfigurations": []any{map[string]any{"name": "primary", "properties": map[string]any{"privateIPAllocationMethod": "Dynamic", "subnet": map[string]string{"id": fmt.Sprintf("[resourceId('Microsoft.Network/virtualNetworks/subnets','%s','luci')]", vnet)}, "publicIPAddress": map[string]string{"id": rid("Microsoft.Network/publicIPAddresses", ip)}}}}}, []string{rid("Microsoft.Network/virtualNetworks", vnet), rid("Microsoft.Network/networkSecurityGroups", nsg), rid("Microsoft.Network/publicIPAddresses", ip)}))
	resources = append(resources, resource("Microsoft.Compute/virtualMachines", "2024-03-01", r.Name, map[string]any{
		"hardwareProfile": map[string]string{"vmSize": size},
		"storageProfile":  map[string]any{"imageReference": map[string]string{"publisher": "Canonical", "offer": "ubuntu-24_04-lts", "sku": "server", "version": "latest"}, "osDisk": map[string]any{"createOption": "FromImage", "diskSizeGB": 64, "managedDisk": map[string]string{"storageAccountType": "StandardSSD_LRS"}}},
		"osProfile":       map[string]any{"computerName": r.Name, "adminUsername": "luci", "customData": base64.StdEncoding.EncodeToString([]byte(bootstrap)), "linuxConfiguration": map[string]any{"disablePasswordAuthentication": true, "ssh": map[string]any{"publicKeys": []any{map[string]string{"path": "/home/luci/.ssh/authorized_keys", "keyData": r.SSHPublicKey}}}}},
		"networkProfile":  map[string]any{"networkInterfaces": []any{map[string]string{"id": rid("Microsoft.Network/networkInterfaces", nic)}}},
	}, []string{rid("Microsoft.Network/networkInterfaces", nic)}))
	return map[string]any{"$schema": "https://schema.management.azure.com/schemas/2019-04-01/deploymentTemplate.json#", "contentVersion": "1.0.0.0", "resources": resources, "outputs": map[string]any{"publicIp": map[string]string{"type": "string", "value": fmt.Sprintf("[reference(resourceId('Microsoft.Network/publicIPAddresses', '%s'), '2023-11-01').ipAddress]", ip)}, "domain": map[string]string{"type": "string", "value": r.Domain}}}
}
