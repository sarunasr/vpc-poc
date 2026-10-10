.PHONY: test collections collections-build operator-test ansible-image operator-image terraform-fmt terraform-validate

test: collections operator-test terraform-fmt

operator-test:
	cd operator && go test ./...

collections:
	$(MAKE) collections-build
	ANSIBLE_ROLES_PATH=$(CURDIR)/ansible/collections/ansible_collections/homelab/web/roles ansible-playbook --syntax-check -i localhost, ansible/collections/ansible_collections/homelab/web/roles/nginx/tests/test.yml
	ANSIBLE_ROLES_PATH=$(CURDIR)/ansible/collections/ansible_collections/homelab/system/roles ansible-playbook --syntax-check -i localhost, ansible/collections/ansible_collections/homelab/system/roles/textfile/tests/test.yml

collections-build:
	mkdir -p ansible/collections/dist
	ansible-galaxy collection build --force ansible/collections/ansible_collections/homelab/web --output-path ansible/collections/dist
	ansible-galaxy collection build --force ansible/collections/ansible_collections/homelab/system --output-path ansible/collections/dist

ansible-image: collections-build
	docker build --platform linux/amd64 -f ansible/ee/Dockerfile -t ghcr.io/sarunasr/vpc-poc-ansible-ee:1.0.2 .

operator-image:
	docker build --platform linux/amd64 -f operator/Dockerfile -t ghcr.io/sarunasr/vpc-poc-operator:1.0.2 .

terraform-fmt:
	terraform fmt -check -recursive terraform

terraform-validate:
	terraform -chdir=terraform/environments/homelab init -backend=false
	terraform -chdir=terraform/environments/homelab validate
