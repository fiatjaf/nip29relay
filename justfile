build:
    CC=$(which musl-gcc) go build -ldflags='-s -w -linkmode external -extldflags "-static"' -o ./n29

deploy: build
    ssh root@turgot 'systemctl stop n29'
    rsync n29 turgot:n29/n29-new
    ssh turgot 'mv n29/n29-new n29/n29'
    ssh root@turgot 'systemctl start n29'
