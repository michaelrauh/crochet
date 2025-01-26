terraform {
  backend "s3" {
    bucket                      = "crochet-tf-state"
    key                         = "terraform/state/crochet.tfstate"
    region                      = "us-east-1"  
    
    endpoints = {
      s3 = "https://nyc3.digitaloceanspaces.com"
    }

    skip_credentials_validation = true
    skip_metadata_api_check     = true
    use_path_style              = true
    encrypt                     = true
    skip_requesting_account_id  = true
  }
}