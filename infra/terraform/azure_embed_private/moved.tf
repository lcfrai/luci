# Preserve existing module state addresses after adding the existing-VNet and
# separately owned foundation fast paths. Never recreate an owned network only
# because the optional-resource address now has an index.
moved {
  from = azurerm_virtual_network.private
  to   = azurerm_virtual_network.private[0]
}
moved {
  from = azurerm_private_dns_zone.vault
  to   = azurerm_private_dns_zone.vault[0]
}
moved {
  from = azurerm_private_dns_zone_virtual_network_link.vault
  to   = azurerm_private_dns_zone_virtual_network_link.vault[0]
}
moved {
  from = azurerm_private_endpoint.vault
  to   = azurerm_private_endpoint.vault[0]
}
moved {
  from = azurerm_private_dns_zone.blob
  to   = azurerm_private_dns_zone.blob[0]
}
moved {
  from = azurerm_private_dns_zone_virtual_network_link.blob
  to   = azurerm_private_dns_zone_virtual_network_link.blob[0]
}
