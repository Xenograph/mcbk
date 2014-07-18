#!/usr/bin/gorun
package main

import (
	"bufio"
	"errors"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

//Do not use trailing slashes
const (
	BACKUP_ROOT            = ""                                                         //Path to save backups in
	BACKUP_DIR_PREFIX      = "minecraft"                                                //Prefix for backup dir names. Suffix is month-year
	BUP_BRANCH_NAME        = "minecraft_server"                                         //Branch name to use with bup
	LOG_PATH               = BACKUP_ROOT + "/" + BACKUP_DIR_PREFIX + "_" + "backup.log" //Path to logfile for this script
	SCREEN_SESSION         = "minecraft"                                                //Session where your minecraft server is running
	MINECRAFT_LOG_PATH     = ""                                                         //Path to minecraft server log
	MINECRAFT_DIR          = ""                                                         //The directory to be backed up
	VERIFY_COMMAND_TIMEOUT = 10 * time.Second                                           //May need to be adjusted for saving large worlds
)

var logger *log.Logger

func main() {
	err := initLogger()
	if err != nil {
		println("ERROR OPENING LOG FILE:", err.Error())
		os.Exit(1)
	}

	if !isMinecraftAlive() {
		//Silently exit, nothing to do if minecraft won't respond
		os.Exit(1)
	}

	defer func() {
		err := sendCommandAndVerify("save-on", "Turned on world auto-saving")
		if err != nil {
			logger.Println("Error turning world saving back on:", err.Error())
		}
	}()

	sayMessage("Backing up world...")

	err = sendCommandAndVerify("save-off", "Turned off world auto-saving")
	if err != nil {
		logger.Println("Error turning off world saving:", err.Error())
		return
	}

	logger.Println("Saving minecraft world...")
	err = sendCommandAndVerify("save-all", "Saved the world")
	if err != nil {
		logger.Println("Error saving world:", err.Error())
		return
	}

	logger.Println("Backing up...")
	err = doBupBackup()
	if err != nil {
		logger.Println("Error saving backup:", err.Error())
		return
	}

	sayMessage("Backup complete")

	logger.Println("Pruning old backups...")
	err = pruneOldBackups()
	if err != nil {
		logger.Println("Error pruning old backups:", err.Error())
	}
}

// Initializes the global variable (gasp) for the logger
func initLogger() error {
	f, err := os.OpenFile(LOG_PATH, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	logger = log.New(f, "", log.LstdFlags)
	return nil
}

// Quick check to see if the minecraft server is alive and responsive
func isMinecraftAlive() bool {
	return sendCommandAndVerify("list", "players online") == nil
}

// Checks if the file or directory at the given path exists
func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// Does the actual backup portion
func doBupBackup() error {
	err := createBackupDirIfNeeded()
	if err != nil {
		return err
	}
	bupPath := getCurrentBupRepoPath()
	cmd := exec.Command("bup", "-d", bupPath, "index", MINECRAFT_DIR)
	err = cmd.Run()
	if err != nil {
		return err
	}

	cmd = exec.Command("bup", "-d", bupPath, "save", "-n", BUP_BRANCH_NAME, MINECRAFT_DIR)
	err = cmd.Run()
	if err != nil {
		return err
	}
	return nil
}

// Creates and initializes the current month's bup repo directory, in the
// case that it does not exist.
func createBackupDirIfNeeded() error {
	bupPath := getCurrentBupRepoPath()
	dirExists, err := exists(bupPath)
	if err != nil {
		return err
	}

	if !dirExists {
		os.MkdirAll(bupPath, 0770)
		cmd := exec.Command("bup", "-d", bupPath, "init")
		err = cmd.Run()
		if err != nil {
			return err
		}
	}
	return nil
}

// Prunes any old backups, if they exist.
func pruneOldBackups() error {
	bupPath := getBupRepoPathToPrune()
	oldBackupExists, err := exists(bupPath)
	if err != nil {
		return err
	}
	if !oldBackupExists {
		return nil
	}
	return os.RemoveAll(bupPath)
}

// Returns the full path to the current month's bup repo directory.
func getCurrentBupRepoPath() string {
	now := time.Now()
	year, month, _ := now.Date()
	monthNum := int(month)
	return BACKUP_ROOT + "/" + BACKUP_DIR_PREFIX + "-" + strconv.Itoa(monthNum) + "-" + strconv.Itoa(year)
}

// Returns the full path to the bup repo directory that should be pruned,
// which is the repo that is two months old in this case.
func getBupRepoPathToPrune() string {
	now := time.Now()
	before := now.AddDate(0, -2, 0)
	year, month, _ := before.Date()
	monthNum := int(month)
	return BACKUP_ROOT + "/" + BACKUP_DIR_PREFIX + "-" + strconv.Itoa(monthNum) + "-" + strconv.Itoa(year)
}

func sendCommand(command string) error {
	cmd := exec.Command("screen", "-S", SCREEN_SESSION, "-p", "0", "-X", "stuff", command+"\\r")
	return cmd.Run()
}

// Sends the given command string to the minecraft server and looks
// for the the substring match in the server log output to confirm
// that the command was sucessfully executed.
func sendCommandAndVerify(command, match string) error {
	cmd := exec.Command("tail", "-n", "0", "-F", MINECRAFT_LOG_PATH)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	defer stdout.Close()

	buffer := bufio.NewReader(stdout)
	cmd.Start()
	defer cmd.Process.Kill()

	ch := make(chan error, 1)
	go func() {
		for {
			line, err := buffer.ReadString('\n')
			if err != nil {
				ch <- err
				break
			}
			matched := strings.Contains(line, match)
			if matched {
				ch <- nil
				break
			}
		}
	}()

	sendCommand(command)

	select {
	case err = <-ch:
		return err
	case <-time.After(VERIFY_COMMAND_TIMEOUT):
		return errors.New("Command verification timeout")
	}
	panic("unreachable")
}

// Attempts to say a global message on the minecraft server without verifying
// that it was sent
func sayMessage(msg string) {
	sendCommand("say " + msg)
}
