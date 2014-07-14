A simple minecraft server backup script that uses [bup](https://github.com/bup/bup) for incremental backups to conserve space. By 
default, it will create one bup repository for each month of backups, and delete a repository once is becomes two months old.

This script is written in Go and meant to be used like a normal shell script with the help of [gorun](https://launchpad.net/gorun).
Be sure to edit the script to configure the paths for your server.

The only dependencies are Go, gorun, and bup.
