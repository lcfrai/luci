{{/*
Common labels for every object this chart renders.
*/}}
{{- define "platform.labels" -}}
app.kubernetes.io/part-of: panacea-app
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{/*
Pod Security Admission: `restricted`, enforced, audited and warned, version-pinned to
`latest` so a cluster upgrade tightens the namespace rather than leaving it on an old
profile. Task 14's sandbox pod (non-root, no escalation, all capabilities dropped,
RuntimeDefault seccomp, emptyDir / PVC / secret volumes only) passes; a regression is
refused by the API server, not merely missed by a review.
*/}}
{{- define "platform.podSecurityLabels" -}}
pod-security.kubernetes.io/enforce: restricted
pod-security.kubernetes.io/enforce-version: latest
pod-security.kubernetes.io/audit: restricted
pod-security.kubernetes.io/audit-version: latest
pod-security.kubernetes.io/warn: restricted
pod-security.kubernetes.io/warn-version: latest
{{- end -}}

{{/*
A CIDR from the foundation outputs. Empty fails the render: values.schema.json rejects it
first on `helm lint`, `template` and `install`, and this is the second gate, so a render
that somehow reaches the templates with an empty CIDR still stops rather than emitting an
empty `except` entry. Call as: include "platform.cidr" (dict "key" "network.vnetCidr"
"value" .Values.network.vnetCidr)
*/}}
{{- define "platform.cidr" -}}
{{- required (printf "%s must be a CIDR from the foundation outputs; it is empty" .key) .value -}}
{{- end -}}

{{/*
True when the CIDR lies wholly inside one of the three RFC 1918 blocks, which
`egress-internet` already excludes. Containment needs both the network address to fall in
the block AND the prefix to be no shorter than the block's, so a straddling CIDR such as
10.0.0.0/7 is correctly reported as NOT contained and is excluded explicitly.
Returns "true" or "".
*/}}
{{- define "platform.isRfc1918" -}}
{{- $parts := splitList "/" . -}}
{{- $octets := splitList "." (index $parts 0) -}}
{{- $o1 := atoi (index $octets 0) -}}
{{- $o2 := atoi (index $octets 1) -}}
{{- $prefix := atoi (index $parts 1) -}}
{{- if and (eq $o1 10) (ge $prefix 8) -}}true
{{- else if and (eq $o1 172) (ge $o2 16) (le $o2 31) (ge $prefix 12) -}}true
{{- else if and (eq $o1 192) (eq $o2 168) (ge $prefix 16) -}}true
{{- end -}}
{{- end -}}

{{/*
The `except` list for `egress-internet`: the Azure metadata endpoint, the Azure host's DNS
and wire server, the three RFC 1918 blocks, and each foundation CIDR that lies outside
them. Cilium applies `ipBlock` to traffic leaving the cluster and matches in-cluster pods
by identity, so the pod-to-pod deny does not depend on this list - that this is how AKS's
managed Cilium treats `ipBlock` is training knowledge, UNVERIFIED; task 32 probes every
deny from inside a real Kata pod.
*/}}
{{- define "platform.egressExcept" -}}
{{- $out := list "169.254.169.254/32" "168.63.129.16/32" "10.0.0.0/8" "172.16.0.0/12" "192.168.0.0/16" -}}
{{- range $key, $cidr := .Values.network -}}
{{- $v := include "platform.cidr" (dict "key" (printf "network.%s" $key) "value" $cidr) -}}
{{- if not (include "platform.isRfc1918" $v) -}}
{{- $out = append $out $v -}}
{{- end -}}
{{- end -}}
{{- toYaml (uniq $out) -}}
{{- end -}}
