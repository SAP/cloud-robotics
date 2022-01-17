# FILE FOR DOCKER proto_generator
# compiles any proto file in the /proto folder. To add more proto files, create a corresponding sub folder in /proto
# (see http-relay/http_over_rpc.proto as an example)
for d in */ ; do
    for f in "$d"*.proto ; do
        #protoc -I="$d" --go_out=../ "$f"
        /protoc/bin/protoc -I="$d" --go_out=../ "$f"
    done
done
