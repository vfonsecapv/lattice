package command_factory

import (
	"fmt"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/cloudfoundry/gunk/timeprovider"
	"github.com/codegangsta/cli"
	"github.com/pivotal-cf-experimental/lattice-cli/app_examiner"
	"github.com/pivotal-cf-experimental/lattice-cli/app_examiner/command_factory/presentation"
	"github.com/pivotal-cf-experimental/lattice-cli/colors"
	"github.com/pivotal-cf-experimental/lattice-cli/output"
	"github.com/pivotal-cf-experimental/lattice-cli/output/cursor"
)

type AppExaminerCommandFactory struct {
	appExaminerCommand *appExaminerCommand
}

type exitHandler interface {
	OnExit(func())
}

func NewAppExaminerCommandFactory(appExaminer app_examiner.AppExaminer, output *output.Output, timeProvider timeprovider.TimeProvider, exitHandler exitHandler) *AppExaminerCommandFactory {
	return &AppExaminerCommandFactory{&appExaminerCommand{appExaminer, output, timeProvider, exitHandler}}
}

func (commandFactory *AppExaminerCommandFactory) MakeListAppCommand() cli.Command {

	var startCommand = cli.Command{
		Name:        "list",
		ShortName:   "li",
		Description: "List all apps on lattice.",
		Usage:       "ltc list",
		Action:      commandFactory.appExaminerCommand.listApps,
		Flags:       []cli.Flag{},
	}

	return startCommand
}

func (commandFactory *AppExaminerCommandFactory) MakeVisualizeCommand() cli.Command {

	var visualizeFlags = []cli.Flag{
		cli.DurationFlag{
			Name:  "rate, r",
			Usage: "The rate in seconds at which to refresh the visualization.",
		},
	}

	var startCommand = cli.Command{
		Name:        "visualize",
		Description: "Visualize Lattice Cells",
		Usage:       "ltc visualize",
		Action:      commandFactory.appExaminerCommand.visualizeCells,
		Flags:       visualizeFlags,
	}

	return startCommand
}

func (commandFactory *AppExaminerCommandFactory) MakeStatusCommand() cli.Command {
	return cli.Command{
		Name:        "status",
		Description: "Displays detailed status information about the app and its instances",
		Usage:       "ltc status",
		Action:      commandFactory.appExaminerCommand.appStatus,
		Flags:       []cli.Flag{},
	}
}

type appExaminerCommand struct {
	appExaminer  app_examiner.AppExaminer
	output       *output.Output
	timeProvider timeprovider.TimeProvider
	exitHandler  exitHandler
}

func (cmd *appExaminerCommand) listApps(context *cli.Context) {
	appList, err := cmd.appExaminer.ListApps()
	if err != nil {
		cmd.output.Say("Error listing apps: " + err.Error())
		return
	} else if len(appList) == 0 {
		cmd.output.Say("No apps to display.")
		return
	}

	w := &tabwriter.Writer{}
	w.Init(cmd.output, 10+colors.ColorCodeLength, 8, 1, '\t', 0)

	header := fmt.Sprintf("%s\t%s\t%s\t%s\t%s", colors.Bold("App Name"), colors.Bold("Instances"), colors.Bold("DiskMB"), colors.Bold("MemoryMB"), colors.Bold("Routes"))
	fmt.Fprintln(w, header)

	for _, appInfo := range appList {
		routes := strings.Join(appInfo.Routes, " ")

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", colors.Bold(appInfo.ProcessGuid), colorInstances(appInfo), colors.NoColor(strconv.Itoa(appInfo.DiskMB)), colors.NoColor(strconv.Itoa(appInfo.MemoryMB)), colors.Cyan(routes))
	}
	w.Flush()
}

func (cmd *appExaminerCommand) appStatus(context *cli.Context) {
	if len(context.Args()) < 1 {
		cmd.output.IncorrectUsage("App Name required")
		return
	}

	appName := context.Args()[0]
	appInfo, err := cmd.appExaminer.AppStatus(appName)

	if err != nil {
		cmd.output.Say(err.Error())
		return
	}

	minColumnWidth := 13
	w := tabwriter.NewWriter(cmd.output, minColumnWidth, 8, 1, '\t', 0)

	horizontalRule := func(pattern string) {
		header := strings.Repeat(pattern, 80) + "\n"
		fmt.Fprintf(w, header)
	}

	headingIndentation := strings.Repeat(" ", minColumnWidth/2)

	titleBar := func(title string) {
		horizontalRule("=")
		fmt.Fprintf(w, "%s%s\n", headingIndentation, title)
		horizontalRule("-")
	}

	titleBar(colors.Bold(appName))

	fmt.Fprintf(w, "%s\t%s\n", "Instances", colorInstances(appInfo))
	fmt.Fprintf(w, "%s\t%s\n", "Stack", appInfo.Stack)

	fmt.Fprintf(w, "%s\t%d\n", "Start Timeout", appInfo.StartTimeout)
	fmt.Fprintf(w, "%s\t%d\n", "DiskMB", appInfo.DiskMB)
	fmt.Fprintf(w, "%s\t%d\n", "MemoryMB", appInfo.MemoryMB)
	fmt.Fprintf(w, "%s\t%d\n", "CPUWeight", appInfo.CPUWeight)

	portStrings := make([]string, 0)
	for _, port := range appInfo.Ports {
		portStrings = append(portStrings, fmt.Sprint(port))
	}

	fmt.Fprintf(w, "%s\t%s\n", "Ports", strings.Join(portStrings, ","))
	fmt.Fprintf(w, "%s\t%s\n", "Routes", strings.Join(appInfo.Routes, " "))
	fmt.Fprintf(w, "%s\t%s\n", "LogGuid", appInfo.LogGuid)
	fmt.Fprintf(w, "%s\t%s\n", "LogSource", appInfo.LogSource)
	fmt.Fprintf(w, "%s\t%s\n", "Annotation", appInfo.Annotation)

	horizontalRule("-")
	var envVars string
	for _, envVar := range appInfo.EnvironmentVariables {
		envVars += envVar.Name + `="` + envVar.Value + `" ` + "\n"
	}
	fmt.Fprintf(w, "%s\n\n%s", "Environment", envVars)

	fmt.Fprintln(w, "")
	horizontalRule("=")

	instanceBar := func(index, state string) {
		fmt.Fprintf(w, "%sInstance %s  [%s]\n", headingIndentation, index, state)
		horizontalRule("-")
	}

	for _, instance := range appInfo.ActualInstances {
		instanceBar(fmt.Sprint(instance.Index), presentation.ColorInstanceState(instance.State))

		fmt.Fprintf(w, "%s\t%s\n", "InstanceGuid", instance.InstanceGuid)
		fmt.Fprintf(w, "%s\t%s\n", "Cell ID", instance.CellID)
		fmt.Fprintf(w, "%s\t%s\n", "Ip", instance.Ip)

		portMappingStrings := make([]string, 0)
		for _, portMapping := range instance.Ports {
			portMappingStrings = append(portMappingStrings, fmt.Sprintf("%d:%d", portMapping.HostPort, portMapping.ContainerPort))
		}
		fmt.Fprintf(w, "%s\t%s\n", "Ports", strings.Join(portMappingStrings, ";"))

		fmt.Fprintf(w, "%s\t%s\n", "Since", fmt.Sprint(instance.Since))

		horizontalRule("-")
	}

	w.Flush()
}

func (cmd *appExaminerCommand) visualizeCells(context *cli.Context) {
	rate := context.Duration("rate")

	cmd.output.Say(colors.Bold("Distribution\n"))
	linesWritten := cmd.printDistribution()

	if rate == 0 {
		return
	}

	closeChan := make(chan bool)
	cmd.output.Say(cursor.Hide())

	cmd.exitHandler.OnExit(func() {
		closeChan <- true
		cmd.output.Say(cursor.Show())
	})

	for {
		select {
		case <-closeChan:
			return
		case <-cmd.timeProvider.NewTimer(rate).C():
			cmd.output.Say(cursor.Up(linesWritten))
			linesWritten = cmd.printDistribution()
		}
	}
}

func (cmd *appExaminerCommand) printDistribution() int {
	defer cmd.output.Say(cursor.ClearToEndOfDisplay())

	cells, err := cmd.appExaminer.ListCells()
	if err != nil {
		cmd.output.Say("Error visualizing: " + err.Error())
		cmd.output.Say(cursor.ClearToEndOfLine())
		cmd.output.NewLine()
		return 1
	}

	for _, cell := range cells {
		cmd.output.Say(cell.CellID)
		if cell.Missing {
			cmd.output.Say(colors.Red("[MISSING]"))
		}
		cmd.output.Say(": ")

		cmd.output.Say(colors.Green(strings.Repeat("•", cell.RunningInstances)))
		cmd.output.Say(colors.Yellow(strings.Repeat("•", cell.ClaimedInstances)))
		cmd.output.Say(cursor.ClearToEndOfLine())
		cmd.output.NewLine()
	}

	return len(cells)
}

func colorInstances(appInfo app_examiner.AppInfo) string {
	instances := fmt.Sprintf("%d/%d", appInfo.ActualRunningInstances, appInfo.DesiredInstances)
	if appInfo.ActualRunningInstances == appInfo.DesiredInstances {
		return colors.Green(instances)
	} else if appInfo.ActualRunningInstances == 0 {
		return colors.Red(instances)
	}

	return colors.Yellow(instances)
}
