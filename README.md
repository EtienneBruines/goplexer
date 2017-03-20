# goplexer
Transparent port-mutiplexer written in Go

## Installation

### Binary Release
Download the binary, and simply run it. No dependencies needed. 

### Source
* `go get github.com/EtienneBruines/goplexer`


## Configuration

Configure a file like `settings.yaml`. Running SSH, HTTP and HTTPS on the same port, would result in this
configuration file:

```yaml
server:
  listen: localhost:8080

services:
  - type: ssh
    keyword: ssh
    location: localhost:22
  - type: http
    keyword: get
    location: localhost:80
  - type: https
    keyword_bytes:
    - 22
      3
      1
      2
      0
      1
      0
      1
      252
      3
      3
    location: localhost:443
```

### Syntax:
- Determining connection type is done by looking at the first few bytes the client transmits:
  - The `keyword_bytes` contains the raw bytes;  
  - Alternatively, `keyword` can be used. This is expected to be fully lowercase.  
- A name for the service can be written in the `type` field.
- The `location` field is the TCP-location that the service will proxy to.
- You can configure the port at which goplexer runs in the `server.listen` field.


## Contributing
Any additional services are welcome (PRs?), as well as bugfixes/bugreports. Feel free to use the Issues-section of GitHub. 
