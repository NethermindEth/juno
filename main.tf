terraform {


  required_providers {
    aws = {
      source = "hashicorp/aws"
    }
  }

 backend "s3" {
   bucket = "custom-terraform-state"
   key = "dev/s3/terraform.tfstate"
   region= "us-west-2"
   encrypt = true
 }
 

}

provider "aws" {
  region = var.selected_region
}


resource "aws_key_pair" "key-module" {
  key_name   = "key-module"
  public_key = var.pub_key
}
/*
resource "aws_s3_bucket" "terraform_state" {
  bucket = "custom-terraform-state"
  lifecycle {
  prevent_destroy = true
  }

  versioning {
  enabled = true
  }

  server_side_encryption_configuration {
  rule {
  apply_server_side_encryption_by_default {
  sse_algorithm = "AES256"
  } 
  }
  }
}
*/



data "template_file" "user_data" {
  template = "${file("./userdata.yaml")}"
}

resource "aws_instance" "web" {
  
  ami           = var.ami_value
  instance_type = var.chosen_instance_type
  key_name = "${aws_key_pair.key-module.key_name}"
  user_data = data.template_file.user_data.rendered
  vpc_security_group_ids = [aws_security_group.sg_web2.id]


  tags = {
    Name = "MyServer"
  }

 
}


output "public_ip" {  
  description = "Public IP address of the EC2 instance"
  value = aws_instance.web.public_ip
}

//This uses the id of the default vpc
data "aws_vpc" "main" {
  id = var.vpc_id
}


resource "aws_security_group" "sg_web2" {
  name        = "sg_web2"
  description = "Security Group"
  vpc_id      = data.aws_vpc.main.id

  ingress = [
    {
    description      = "HTTP"
    from_port        = 80
    to_port          = 80
    protocol         = "tcp"
    cidr_blocks      = ["0.0.0.0/0"]
    ipv6_cidr_blocks = []
    prefix_list_ids = []
    security_groups = []
    self = false
  },
    {
    description      = "SSH"
    from_port        = 22
    to_port          = 22
    protocol         = "tcp"
    cidr_blocks      = ["0.0.0.0/0"]
    ipv6_cidr_blocks = []
    prefix_list_ids = []
    security_groups = []
    self = false
  }

]
  egress = [
    { 
    description      = "outgoing traffic"
    from_port        = 0
    to_port          = 0
    protocol         = "-1"
    cidr_blocks      = ["0.0.0.0/0"]
    ipv6_cidr_blocks = ["::/0"]
    prefix_list_ids = []
    security_groups = []
    self = false
  }

]

}



