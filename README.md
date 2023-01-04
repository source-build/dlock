# Single node distributed lock

## Docker

```shell
docker pull grxc2312/dlock:v1
docker run -d -p 7668:7668  -v ~/dlock_logs:/logs grxc2312/dlock:v1 --server.port=7668 --server.secretKey="xxx"
```

The same secretKey is required for client access

## Usage

[go-dlock](https://github.com/source-build/go-dlock.git)