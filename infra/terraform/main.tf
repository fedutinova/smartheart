resource "hcloud_ssh_key" "smartheart" {
  name       = "smartheart-${var.environment}"
  public_key = var.ssh_public_key
}

resource "hcloud_firewall" "smartheart" {
  name = "smartheart-${var.environment}"

  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "22"
    source_ips = ["0.0.0.0/0", "::/0"]
  }

  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "80"
    source_ips = ["0.0.0.0/0", "::/0"]
  }

  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "443"
    source_ips = ["0.0.0.0/0", "::/0"]
  }

  rule {
    direction  = "in"
    protocol   = "udp"
    port       = "443"
    source_ips = ["0.0.0.0/0", "::/0"]
  }
}

resource "hcloud_server" "smartheart" {
  name         = "smartheart-${var.environment}"
  server_type  = var.server_type
  image        = "ubuntu-24.04"
  location     = var.server_location
  ssh_keys     = [hcloud_ssh_key.smartheart.id]
  firewall_ids = [hcloud_firewall.smartheart.id]
  user_data    = file("${path.module}/user_data.sh")

  labels = {
    project     = "smartheart"
    environment = var.environment
  }
}
