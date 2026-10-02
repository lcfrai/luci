# aks_cluster_id is intentionally omitted here. Azure resource ids are
# environment-driven, never a checked-in placeholder. Supply it at
# plan/apply time, e.g.
#   export TF_VAR_aks_cluster_id=$(az aks show -g rg-aks-luci-dev -n aks-luci-dev --query id -o tsv)
node_count = 1
max_count  = 4
vm_size    = "Standard_D8s_v5"
