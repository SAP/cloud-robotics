#!/usr/bin/env bash
#
# Copyright 2021 The Cloud Robotics Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# If using linux with a WSL 1 backend and docker via an alias
{ var="$(docker)"; } &> /dev/null
if [[ "$var" =~ "The command 'docker' could not be found in this WSL 1 distro" ]]
then
    shopt -s expand_aliases
    source ~/.bash_aliases # common in Ubuntu
    echo "expanded aliases to include docker command for WSL 1."
fi
# --------------



# directory of this script
dir="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

# build proto files and return image id (via -q)
echo "building build image...(this may take a while)"
image_id=$(docker build -q --target proto_generator "${dir}/..")

# create docker container containing .pb.go files and copy them to this repository
echo "creating container and copying files..."
container_id=$(docker create "$image_id")

# docker cp "$container_id:/code/src/proto" "$dir/../src" # would also work but copy unnecessary files as well (and attempt to copy src files)
echo "Copying generated protobuf files"
for d in "$dir"/../src/proto/*/ ; do
    for f in "$d"*.proto ; do
        file="$(basename "$d")/$(basename "${f%proto}pb.go")"
        echo "copying: ${file}"
        docker cp "$container_id:/code/src/proto/${file}" "$d"
    done
done

echo "Copying generated k8s api deepcopy files"
for d in "$dir"/../src/go/pkg/apis/*/*/ ; do
    for f in "$d"doc.go ; do
        file="$(basename $(dirname "$d"))/$(basename "$d")/zz_generated.deepcopy.go"
        echo "copying: ${file}"
        docker cp "$container_id:/code/src/go/pkg/apis/${file}" "$d"
    done
done

echo "Copying k8s clients"
rm -rf "${dir}/../src/go/pkg/client"
docker cp "$container_id:/code/src/go/pkg/client" "${dir}/../src/go/pkg/"

# clean up
echo "cleaning up..."
docker rm "$container_id"
docker image rm "$image_id"
