.PHONY: all cli ctrl webhook

all:
	kubectl apply -f ./cli/deployment.yml
	kubectl apply -f ./ctrl/deployment.yml
	kubectl apply -f ./ctrl/service.yml
	kubectl apply -f ./webhook/webhook.yml

cli:
	kubectl apply -f ./cli/deployment.yml

ctrl:
	kubectl apply -f ./ctrl/deployment.yml
	kubectl apply -f ./ctrl/service.yml

webhook:
	kubectl apply -f ./webhook/webhook.yml