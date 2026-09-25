# infra/terraform/backends/prod.hcl — what every stack passes as -backend-config=…/prod.hcl.
# The account name and resource group are NOT here: the pipeline passes them from bootstrap's
# outputs as further -backend-config key=value flags, so this file holds no environment identity
# beyond the container and the auth mode. key is passed per stack: <stack>.<env>.tfstate.
container_name   = "tfstate"
use_azuread_auth = true
