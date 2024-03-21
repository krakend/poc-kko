How To Run This
=========

# Install minikube

https://minikube.sigs.k8s.io/docs/start/

In a console, execute `minikube start && minikube dashboard`

# KrakenD controller

In the root of this repo:

1. Set the envar: `export KRAKEND_IMAGE="docker.io/devopsfaith/krakend:2.6-watch"`
2. Deploy and run: `make deploy run'