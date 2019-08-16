oauth2-proxy-manager
============================

Setup
=====================================
## 0. Install ingress-nginx(nginx-ingress) in your cluster.
> Helm chart: https://github.com/helm/charts/tree/master/stable/nginx-ingress

## 1. GitHub
### 1-1. Create OAuth Application
* Authorization callback URL (ex, `https://auth.example.com/github` )
> https://github.com/settings/applications/new
> ![](https://i.imgur.com/lbxkHXg.png)

## 2. Setup Secret
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: oauth2-proxy-manager-secret
  namespace: oauth2-proxy
type: Opaque
stringData:
  OAUTH2_PROXY_CLIENT_ID: "xxxxxxx"
  OAUTH2_PROXY_CLIENT_SECRET: "yyyyyy"
  COOKIE_SALT: "U3VzaGkgaXMgR29kLiBCZSBFYXQgU3VzaGkuCg==" # randomized secret strings.
```
> Another manifests can be see: `/kubernetes` directory.


How to restrict my service?
=====================================
## Example: `supersecret` app
### Fill annotations, and host.
```yaml
apiVersion: extensions/v1beta1
kind: Ingress
metadata:
  name: supersecret
  namespace: supersecret
  annotations:
    # must be use ingress-nginx.
    kubernetes.io/ingress.class: nginx

    # https://auth.example.com/<PROVIDER>/<APP_NAME>/.....
    nginx.ingress.kubernetes.io/auth-signin: https://auth.example.com/github/supersecret/start?rd=https://$host$request_uri$is_args$args
    nginx.ingress.kubernetes.io/auth-url: https://auth.example.com/github/supersecret/auth

    # app-name should be unique.
    oauth2-proxy-manager.k8s.io/app-name: "supersecret"

    # GitHub org, teams
    oauth2-proxy-manager.k8s.io/github-org: "example-corp"
    oauth2-proxy-manager.k8s.io/github-teams: "administrator"
spec:
  rules:
  - host: "supersecret.example.com" # hosts must be provide
    http:
      paths:
      - path: /
        backend:
          serviceName: supersecret
          servicePort: 80
```

## Tada! 🎉