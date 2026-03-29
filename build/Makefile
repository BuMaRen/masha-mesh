.PHONY: all cli ctrl

all:
	kubectl apply -f ./cli/deployment.yml
	kubectl apply -f ./ctrl/mesh-ctrl-pod.yaml

cli:
	kubectl apply -f ./cli/deployment.yml

ctrl:
	kubectl apply -f ./ctrl/mesh-ctrl-pod.yaml