#! /bin/bash

go build
mv pitilt-api db/provisioning/roles/deploy/files
cd db
ansible-playbook -K -i provisioning/inventory provisioning/playbook.yml --tags "deploy_lite"
cd ..