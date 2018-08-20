# sidecar-proxy

A minimal HTTP proxy that supports basic authentication and a "white listing" mechanism for
exposing open endpoints in sidecar deployments for inter-service communication in
containerized environments such as Kubernetes. Though much more narrow in scope, this was 
written in the vein of existing transparent proxies like [Linkerd](https://linkerd.io/) and
[Envoy](https://www.envoyproxy.io), and also inspired by L7 policy filtering features available
in [Cilium](http://http://docs.cilium.io/). It is expected that an existing service mesh 
solution will replace this sidecar proxy.

## Example

Assuming an unprotected application is running in the same Pod as the sidecar proxy on port
9000, point the proxy to the destination `http://localhost:9000` and provide a comma-separated
list of basic auth credentials and open endpoints as environment variables:
```
$ export BASIC_AUTH_ALLOWED="serviceA:password,serviceB:passwurd"
$ export OPEN_ENDPOINTS="/ping,/metrics"

$ ./sidecar-proxy -dest=http://localhost:9000
```
By default the proxy will bind to port 8080, but this can be overriden with the `-addr`
flag. Now try it out with:
```
$ curl target-service:8080/ping
pong

$ curl target-service:8080/protected
not authorized

$ curl target-service:8080/protected -u serviceA:password
{"msg": "protected resource"}
```
Of course SREs/operators should encrypt all `BASIC_AUTH_ALLOWED` pairs as secrets at deploy
time.

