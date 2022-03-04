## Setting up Systemd

If you want to deploy your bbgo binary to a linux system, you could use the systemd to launch your daemon.

To do this, add service file into the directory `/etc/systemd/system/bbgo.service` with the following content:

```shell
[Unit]
After=network-online.target
Wants=network-online.target

[Install]
WantedBy=multi-user.target

[Service]
WorkingDirectory=/home/bbgo
# KillMode=process
ExecStart=/home/bbgo/bbgo run
User=bbgo
Restart=always
RestartSec=60
```

Then, to load the service file, you need to run:

```shell
systemctl daemon-reload
```

And then you can start your service by running enable and start:

```shell
systemctl enable bbgo.service
systemctl start bbgo.service
```

To stop your service, you can run:

```shell
systemctl stop bbgo.service
```

