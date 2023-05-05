build:
    CC=$(which musl-gcc) go build -ldflags='-s -w -linkmode external -extldflags "-static"' -o ./groupsrelay

deploy: build
    ssh root@turgot 'systemctl stop groupsrelay'
    rsync groupsrelay turgot:groupsrelay/groupsrelay-new
    ssh turgot 'mv groupsrelay/groupsrelay-new groupsrelay/groupsrelay'
    ssh root@turgot 'systemctl start groupsrelay'
