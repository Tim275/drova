# Local KEDA autoscaling demo for Drova — trip-service scales on Kafka lag.
# Everything runs in a throwaway k3d cluster on your Mac. No homelab, no cloud.
#
#   make up      # create cluster + KEDA + build/import image + deploy
#   make watch   # (terminal 1) live pods + scaledobject + hpa
#   make load    # (terminal 2) blast messages -> watch trip-service scale 1 -> 3
#   make down    # tear everything down

CLUSTER      ?= drova-keda
NS           ?= drova-local
KEDA_VERSION ?= v2.20.1
REPO_ROOT    := ../..

.PHONY: up cluster keda image deploy load watch hpa logs lag down

up: cluster keda image deploy
	@echo ""
	@echo "✅ Local KEDA stack is up."
	@echo "   Terminal 1:  make watch"
	@echo "   Terminal 2:  make load"

cluster:
	k3d cluster create $(CLUSTER) --no-lb --k3s-arg "--disable=traefik@server:0" --wait
	kubectl cluster-info

keda:
	kubectl apply --server-side -f https://github.com/kedacore/keda/releases/download/$(KEDA_VERSION)/keda-$(patsubst v%,%,$(KEDA_VERSION)).yaml
	# Wait for the CRD, the operator, the metrics-apiserver (the HPA reads its external
	# metric — if not ready the HPA shows <unknown> and SILENTLY never scales) and the
	# admission webhook (the ScaledObject apply is routed through it).
	kubectl wait --for=condition=established crd/scaledobjects.keda.sh --timeout=60s
	kubectl rollout status deployment/keda-operator -n keda --timeout=180s
	kubectl wait --for=condition=Available deploy/keda-metrics-apiserver -n keda --timeout=180s
	kubectl wait --for=condition=Available deploy/keda-admission -n keda --timeout=180s

image:
	# Local arm64-native build — the prod Dockerfile pins amd64 digests that crash
	# under QEMU on Apple Silicon. Context = repo root; Dockerfile sits beside this Makefile.
	docker build -t drova/trip-service:local -f trip-service.local.Dockerfile $(REPO_ROOT)
	k3d image import drova/trip-service:local -c $(CLUSTER)

deploy:
	kubectl apply -k .
	# Cold start needs room: kafka format + trip-service initContainers
	# (wait-postgres, create-topic) + migrations can take a couple of minutes.
	kubectl -n $(NS) rollout status deployment/trip-service --timeout=300s

load:
	kubectl -n $(NS) delete job loadgen --ignore-not-found
	kubectl -n $(NS) apply -f 50-loadgen-job.yaml
	@echo "load job started — watch scaling with 'make watch'"

watch:
	@while true; do \
		clear; \
		echo "== trip-service pods =="; \
		kubectl -n $(NS) get pods -l app=trip-service; \
		echo; echo "== ScaledObject / HPA =="; \
		kubectl -n $(NS) get scaledobject,hpa; \
		sleep 2; \
	done

hpa:
	kubectl -n $(NS) get hpa -w

lag:
	kubectl -n $(NS) exec deploy/kafka -- \
		kafka-consumer-groups --bootstrap-server localhost:9092 \
		--describe --group trip-service-driver-response

logs:
	kubectl -n $(NS) logs -l app=trip-service --tail=40 -f

down:
	k3d cluster delete $(CLUSTER)
