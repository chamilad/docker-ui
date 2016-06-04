#!/usr/bin/env bash

set -e
echo

function echoDim () {
    if [ -z "$2" ]; then
        echo $'\e[2m'"${1}"$'\e[0m'
    else
        echo -n $'\e[2m'"${1}"$'\e[0m'
    fi
}

function echoError () {
    echo -n $'\e[1;31m'"${1}"$'\e[0m'
}

function echoSuccess () {
    echo -n $'\e[1;32m'"${1}"$'\e[0m'
}

function echoDot () {
    echoDim "." "append"
}

function echoBold () {
    echo $'\e[1m'"${1}"$'\e[0m'
}

function askBold () {
    echo -n $'\e[1m'"${1}"$'\e[0m'
}


# Check flags
while getopts :n:p:u: FLAG; do
    case $FLAG in
        n)
            dns=$OPTARG
            ;;
        p)
            server_port=$OPTARG
            ;;
        u)
            api_username=$OPTARG
            ;;
        \?)
            showUsageAndExit
            ;;
    esac
done

trap "echoSuccess COMPLETED!; exit 0;" SIGINT

# Get auth details
if [ ! -z $api_username ]; then
    askBold "Enter password for Docker API: "
    read -rs api_password
    echo
    echo
fi

# Default server port
if [ -z $server_port ]; then
    server_port=8080
fi

# Default nameserver
if [ -z $dns ]; then
    dns=$(grep nameserver /etc/resolv.conf  | head -n1 | awk '{print $2}')
fi

echo -ne "\t.... Building Go statically linked binary"
{
    CGO_ENABLED=0 go build -a --ldflags '-s --extldflags "-static"' server.go && { echo -ne "\r"; echoSuccess "[DONE]"; echo -e "\t.... Building Go statically linked binary"; }
} || {
    echo -ne "\r"
    echoError "[FAILED]"
    echo -e "\t.... Building Go statically linked binary"
    echo
    exit 1
}


echo -ne "\t.... Building Docker image"
docker build -t chamilad/docker-ui . > /dev/null 2>&1
image_details=$(docker images chamilad/docker-ui | tail -1 | awk '{print $1 ":" $2 "(" $3 "), size - " $(NF-1) $NF}')
echo -ne "\r"
echoSuccess "[DONE]"
echo -e "\t.... Building Docker image ${image_details}"

echo -ne "\t.... Cleaning"
rm -rf server
echo -ne "\r"
echoSuccess "[DONE]"
echo -e "\t.... Cleaning"

echo
echo "Running Docker UI..."
#docker_api_host_ip=$(dig +short dockerhub.private.wso2.com)
docker_api_host_ip="dockerhub.private.wso2.com"

{
docker run -i \
    --dns-search=private.wso2.com \
    --dns="${dns}" \
    -e DOCKERUI_API_HOST="${docker_api_host_ip}" \
    -e DOCKERUI_API_SKIP_VERIFICATION=false \
    -e DOCKERUI_API_CACRT=ca.crt \
    -e DOCKERUI_PORT="${server_port}" \
    -e DOCKERUI_USERNAME="${api_username}" \
    -e DOCKERUI_PASSWORD="${api_password}" \
    -p $server_port:$server_port \
    -t chamilad/docker-ui:latest
} || {
    if [ $? == 2 ]; then
        echo
        echoSuccess "COMPLETED!"
        echo
        echo
        exit 0
    else
        echo
        echoError "FAILED!"
        echo
        echo
        exit 1
    fi
}

echo
echoSuccess "COMPLETED!"
echo
echo