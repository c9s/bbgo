# Helm Chart

## Requirement

- redis (optional, if you need persistence)
- docker image (you can use the image from docker hub or build one by yourself)

## Install

If you need redis:

```sh
helm repo add bitnami https://charts.bitnami.com/bitnami
helm install redis bitnami/redis
```

To get the dynamically generated redis password, you can use the following command:

```sh
export REDIS_PASSWORD=$(kubectl get secret --namespace bbgo redis -o jsonpath="{.data.redis-password}" | base64 --decode)
```

Prepare your docker image locally (you can also use the docker image from docker hub):

```sh
make docker DOCKER_TAG=1.16.0
```

The docker tag version number is from the file [Chart.yaml](charts/bbgo/Chart.yaml)

Choose your instance name:

```sh
export INSTANCE=grid
```

Prepare your secret:

```sh
kubectl create secret generic bbgo-$INSTANCE --from-env-file .env.local
```

Configure your config file, the chart defaults to read config/bbgo.yaml to create a configmap:

```sh
cp config/grid.yaml bbgo-$INSTANCE.yaml
vim bbgo-$INSTANCE.yaml
```

Prepare your configmap:

```sh
kubectl create configmap bbgo-$INSTANCE --from-file=bbgo.yaml=bbgo-$INSTANCE.yaml
```

Install chart with the preferred release name, the release name maps to the previous secret we just created, that
is, `bbgo-grid`:

```sh
helm install bbgo-$INSTANCE ./charts/bbgo
```

By default, the helm chart uses configmap and dotenv secret by the release name,
if you have an existing configmap that is not named `bbgo-$INSTANCE`, you can specify the configmap via 
the `existingConfigmap` option:

```sh
helm install --set existingConfigmap=bbgo-$INSTANCE bbgo-$INSTANCE ./charts/bbgo
```

To use the latest version:

```sh
helm install --set existingConfigmap=bbgo-$INSTANCE --set image.tag=latest bbgo-$INSTANCE ./charts/bbgo
```

To upgrade:

```sh
helm upgrade bbgo-$INSTANCE ./charts/bbgo
helm upgrade --set image.tag=1.15.2 bbgo-$INSTANCE ./charts/bbgo
```

## Delete an installed chart

```sh
helm delete bbgo-$INSTANCE
```
