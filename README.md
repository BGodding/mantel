# Mantel
This repo is an attempt to create an open source media viewer focused on sharing media with friends and family.

The guide and scripts will be heavily focused on using a Raspberry Pi, but the core services should run fine on most OS's.

## Hasn't this been done dozens of times?
Yes, but there were some features that I could not find in other implementations that I wanted.
High level requirements were:
* Support both video and pictures
* Videos longer than the slide duration will have random slide duration length clips selected for playback
* New media should always be played next, this encourages people to upload new media when gathered near the player.
* (NOT YET DONE) As files becomes stale, show them less often. This is an attempt to handle very large media libraries preventing the latest media from getting a chance to show up.

## Setting up a new  device

The application which handles curating the media for the player is written in GO. You can download the pre-compiled release from #TODO or build from source using the commands below. The example commands are for cross compiling for arm64 linux devices (like the Raspberry Pi).
### Build from source
### *nix
```shell
env GOOS=linux GOARCH=arm64 go build -ldflags "-w"
```
### Windows
Note that env variables must be set as admin when cross compiling
```shell
set GOOS=linux
set GOARCH=arm64
go build -ldflags "-w"
```

## Scp files
```shell
scp mantel setup\rpi-setup.sh services\media-controller.service services\media-player.service msuser@<hostname or ip>:~
dos2unix rpi-setup.sh
chmod +x rpi-setup.sh
sudo -v ; ./rpi-setup.sh
```


## Overclocking
Append the values below to `/boot/firmware/config.txt`, note that the values may need modification.
```
# Tell the DVFS algorithm to increase voltage by this amount (in µV; default 0).
over_voltage_delta=25000
# Set the Arm A76 core frequency (in MHz; default 2400).
arm_freq=2800
```
For more information on improving performance see https://www.jeffgeerling.com/blog/2024/raspberry-pi-boosts-pi-5-performance-sdram-tuning
The lines below can help in validating stability
```
sudo apt install stress-ng mesa-utils
sudo stress-ng --cpu 0 --cpu-method fft
glxgears -fullscreen
while true; do vcgencmd measure_clock arm; vcgencmd measure_temp; sleep 10; done& stress -c 4 -t 900s
```
