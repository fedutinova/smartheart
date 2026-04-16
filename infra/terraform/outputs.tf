output "server_ip" {
  description = "Public IPv4 of the server — point your DNS A records here"
  value       = hcloud_server.smartheart.ipv4_address
}

output "server_ipv6" {
  description = "Public IPv6 of the server"
  value       = hcloud_server.smartheart.ipv6_address
}

output "server_id" {
  description = "Hetzner server ID"
  value       = hcloud_server.smartheart.id
}

output "ssh_command" {
  description = "SSH command to connect to the server"
  value       = "ssh root@${hcloud_server.smartheart.ipv4_address}"
}

output "dns_instructions" {
  description = "DNS records to create at your registrar"
  value       = <<-EOT
    Create these DNS records at your domain registrar:

      ${var.domain_name}       A     ${hcloud_server.smartheart.ipv4_address}
      www.${var.domain_name}   A     ${hcloud_server.smartheart.ipv4_address}
      api.${var.domain_name}   A     ${hcloud_server.smartheart.ipv4_address}

      ${var.domain_name}       AAAA  ${hcloud_server.smartheart.ipv6_address}
      www.${var.domain_name}   AAAA  ${hcloud_server.smartheart.ipv6_address}
      api.${var.domain_name}   AAAA  ${hcloud_server.smartheart.ipv6_address}

    TTL: 300 (or lower for first deploy, raise to 3600 after stable)
  EOT
}
