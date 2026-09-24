# pipelines/tflint.hcl — tflint's configuration for pipelines/infra.yml (task 28). The azurerm
# ruleset is installed and pinned by install-tool.sh (tflint-ruleset-azurerm, SHA-256 there), into
# TFLINT_PLUGIN_DIR; `tflint --init`, which would download it unpinned, is never run. The version
# below must equal that pin. A second cloud's ruleset would be one more pin and one more block.
config {
  call_module_type = "local"
}

plugin "terraform" {
  enabled = true
  preset  = "recommended"
}

plugin "azurerm" {
  enabled = true
  version = "0.32.0"
  source  = "github.com/terraform-linters/tflint-ruleset-azurerm"
}
