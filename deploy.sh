#!/bin/bash
#
# Setup:
#
# 1) Make sure that you have the SSH host called "bbgo" and your linux system has systemd installed.
#
# 2) Use ssh to connect the bbgo host
#
#      $ ssh bbgo
#
# 3) On the REMOTE server, create directory, setup the dotenv file and the bbgo.yaml config file
#      $ mkdir bbgo
#      $ vim bbgo/.env.local
#      $ vim bbgo/bbgo.yaml
#
# 4) Make sure your REMOTE user can use SUDO WITHOUT PASSWORD.
#
# 5) Run the following command to setup systemd from LOCAL:
#
#      $ SETUP_SYSTEMD=yes sh deploy.sh bbgo
#
# 6) To update your existing deployment, simply run deploy.sh again:
#
#      $ sh deploy.sh bbgo
#
set -e

target=$1

# bin_type is the binary type that you want to build bbgo
# use "bbgo" for full-features binary (including web application)
# use "bbgo-slim" for slim version binary (without web application)
bin_type=bbgo-slim

# host_bin_dir is the directory that binary file will be uploaded to.
# default to $HOME/bin
host_bin_dir=bin

host=bbgo
# host_user=ubuntu
# host_home=/root

host_systemd_service_dir=/etc/systemd/system
host_os=linux
host_arch=amd64

# setup_host_systemd_service: should we create a new systemd service file if it does not exist?
# change this to "yes" to enable the automatic setup.
# if setup_host_systemd_service is enabled, the script will create a systemd service file from a template
# and then upload the systemd service file to $host_systemd_service_dir,
# root permission might be needed, you can change the host user to root temporarily while setting up the environment.
setup_host_systemd_service=no
if [[ -n $SETUP_SYSTEMD ]] ; then
    setup_host_systemd_service=yes
fi


use_dnum=no
if [[ -n $USE_DNUM ]] ; then
    use_dnum=yes
fi



# use the git describe as the binary version, you may override this with something else.
tag=$(git describe --tags)

RED='\033[1;31m'
GREEN='\033[1;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

function remote_test() {
  ssh $host "(test $* && echo yes)"
}

function remote_run() {
  ssh $host "$*"
}

function remote_eval() {
  ssh $host "echo $*"
}

function warn() {
  echo "${YELLOW}$*${NC}"
}

function error() {
  echo "${RED}$*${NC}"
}

function info() {
  echo "${GREEN}$@${NC}"
}

if [[ -n $BBGO_HOST ]]; then
  host=$BBGO_HOST
else
  warn "env var BBGO_HOST is not set, using \"bbgo\" host alias as the default host, you can add \"bbgo\" to your ~/.ssh/config file"
  host=bbgo
fi

if [[ -z $target ]]; then
  echo "Usage: $0 [target]"
  echo "target name is required"
  exit 1
fi

# initialize the remote environment
# create the directory for placing binaries
ssh $host "mkdir -p \$HOME/$host_bin_dir && mkdir -p \$HOME/$target"

if [[ $(remote_test "-e $host_systemd_service_dir/$target.service") != "yes" ]]; then
  if [[ "$setup_host_systemd_service" == "no" ]]; then
    error "The systemd $target.service on host $host is not configured, can not deploy"
    exit 1
  fi

  warn "$host_systemd_service_dir/$target.service does not exist, setting up..."

  if [[ -z $host_home ]]; then
    host_home=$(remote_eval "\$HOME")
  fi

  if [[ -z $host_user ]]; then
    host_user=$(remote_eval "\$USER")
  fi


  cat <<END >".systemd.$target.service"
[Unit]
After=network-online.target
Wants=network-online.target

[Install]
WantedBy=multi-user.target

[Service]
WorkingDirectory=$host_home/$target
KillMode=process
ExecStart=$host_home/$target/bbgo run
User=$host_user
Restart=always
RestartSec=30
END

  info "uploading systemd service file..."
  scp ".systemd.$target.service" "$host:$target.service"
  remote_run "sudo mv -v $target.service $host_systemd_service_dir/$target.service"
  # scp ".systemd.$target.service" "$host:$host_systemd_service_dir/$target.service"

  info "reloading systemd daemon..."
  remote_run "sudo systemctl daemon-reload && sudo systemctl enable $target"
fi


bin_target=$bin_type-$host_os-$host_arch

if [[ "$use_dnum" == "yes" ]]; then
  bin_target=$bin_type-dnum-$host_os-$host_arch
fi


info "building binary: $bin_target..."
make $bin_target

# copy the binary to the server
info "deploying..."
info "copying binary to host $host..."

if [[ $(remote_test "-e $host_bin_dir/bbgo-$tag") != "yes" ]] ; then
  scp build/bbgo/$bin_target $host:$host_bin_dir/bbgo-$tag
else
  info "binary $host_bin_dir/bbgo-$tag already exists, we will use the existing one"
fi

if [[ -n "$UPLOAD_ONLY" ]] ; then
  # link binary and restart the systemd service
  info "linking binary"
  ssh $host "(cd $target && ln -sf \$HOME/$host_bin_dir/bbgo-$tag bbgo)"
else
  info "linking binary and restarting..."
  ssh $host "(cd $target && ln -sf \$HOME/$host_bin_dir/bbgo-$tag bbgo && sudo systemctl restart $target.service)"
fi

info "deployed successfully!"
