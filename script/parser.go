package script

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/sirupsen/logrus"
)

var (
	spaceSep = regexp.MustCompile(`\s`)
	quoteSet = regexp.MustCompile(`[\"\']`)
	cmdSep   = regexp.MustCompile(`\s`)
)

func Parse(reader io.Reader) (*Script, error) {
	logrus.Info("Parsing script file")
	lineScanner := bufio.NewScanner(reader)
	lineScanner.Split(bufio.ScanLines)
	var script Script
	script.Preambles = make(map[string][]Command)
	line := 1
	for lineScanner.Scan() {
		text := strings.TrimSpace(lineScanner.Text())
		if text == "" || text[0] == '#' {
			line++
			continue
		}
		logrus.Debugf("Parsing [%d: %s]", line, text)
		tokens := cmdSep.Split(text, -1)
		cmdName := tokens[0]
		if !Cmds[cmdName].Supported {
			return nil, fmt.Errorf("line %d: %s unsupported", line, cmdName)
		}
		// TODO additional validation needed:
		// 1) validate preambles and args
		// 2) validate each action and args
		switch cmdName {
		case CmdAs:
			cmd, err := NewAsCommand(line, tokens[1:])
			if err != nil {
				return nil, err
			}
			script.Preambles[CmdAs] = []Command{cmd} // save only last AS instruction
		case CmdEnv:
			cmd, err := NewEnvCommand(line, tokens[1:])
			if err != nil {
				return nil, err
			}
			script.Preambles[CmdEnv] = append(script.Preambles[CmdEnv], cmd)
		case CmdFrom:
			cmd, err := NewFromCommand(line, tokens[1:])
			if err != nil {
				return nil, err
			}
			script.Preambles[CmdFrom] = []Command{cmd}
		case CmdKubeConfig:
			cmd, err := NewKubeConfigCommand(line, tokens[1:])
			if err != nil {
				return nil, err
			}
			script.Preambles[CmdKubeConfig] = []Command{cmd}
		case CmdWorkDir:
			cmd, err := NewWorkdirCommand(line, tokens[1:])
			if err != nil {
				return nil, err
			}
			script.Preambles[CmdWorkDir] = []Command{cmd}
		case CmdCapture:
			cmdStr := strings.Join(tokens[1:], " ")
			cmd, err := NewCaptureCommand(line, []string{cmdStr})
			if err != nil {
				return nil, err
			}
			script.Actions = append(script.Actions, cmd)
		case CmdCopy:
			cmd, err := NewCopyCommand(line, tokens[1:])
			if err != nil {
				return nil, err
			}
			script.Actions = append(script.Actions, cmd)
		default:
			return nil, fmt.Errorf("%s not supported", cmdName)
		}
		logrus.Debugf("%s parsed OK", cmdName)
		line++
	}
	logrus.Debug("Done parsing")
	return enforceDefaults(&script)
}

func validateCmdArgs(cmdName string, args []string) error {
	cmd, ok := Cmds[cmdName]
	if !ok {
		return fmt.Errorf("%s is unknown", cmdName)
	}

	minArgs := cmd.MinArgs
	maxArgs := cmd.MaxArgs

	if len(args) < minArgs {
		return fmt.Errorf("%s must have at least %d argument(s)", cmdName, minArgs)
	}

	if maxArgs > -1 && len(args) > maxArgs {
		return fmt.Errorf("%s can only have up to %d argument(s)", cmdName, maxArgs)
	}

	return nil
}

func cliParse(cmdStr string) (cmd string, args []string) {
	args = []string{}
	parts := spaceSep.Split(cmdStr, -1)
	if len(parts) == 0 {
		return
	}
	if len(parts) == 1 {
		cmd = parts[0]
		return
	}
	cmd = parts[0]
	args = parts[1:]
	return
}

// enforceDefaults adds missing defaults to the script
func enforceDefaults(script *Script) (*Script, error) {
	if _, ok := script.Preambles[CmdAs]; !ok {
		cmd, err := NewAsCommand(0, []string{fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid())})
		if err != nil {
			return script, err
		}
		script.Preambles[CmdAs] = []Command{cmd}
	}

	if _, ok := script.Preambles[CmdFrom]; !ok {
		cmd, err := NewFromCommand(0, []string{Defaults.FromValue})
		if err != nil {
			return nil, err
		}
		script.Preambles[CmdFrom] = []Command{cmd}
	}

	if _, ok := script.Preambles[CmdWorkDir]; !ok {
		cmd, err := NewWorkdirCommand(0, []string{Defaults.WorkdirValue})
		if err != nil {
			return nil, err
		}
		script.Preambles[CmdWorkDir] = []Command{cmd}
	}

	if _, ok := script.Preambles[CmdKubeConfig]; !ok {
		cmd, err := NewKubeConfigCommand(0, []string{Defaults.KubeConfigValue})
		if err != nil {
			return nil, err
		}
		script.Preambles[CmdKubeConfig] = []Command{cmd}
	}
	return script, nil
}