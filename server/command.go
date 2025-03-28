package main


import (
	"net/http"
	"fmt"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
)

const commandTrigger = "reactions"


// Register all your slash commands in the NewCommandHandler function.
func (p *Plugin) RegisterCommands() {
	err := p.client.SlashCommand.Register(&model.Command{
		Trigger:          commandTrigger,
		AutoComplete:     true,
		AutoCompleteDesc: "Switch on or off",
		AutoCompleteHint: "[on|off]",
	})
	if err != nil {
		p.client.Log.Error("Failed to register command", "error", err)
	}
}

func (p *Plugin) ExecuteCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	cmdArgs := strings.Fields(args.Command)

	if "/"+commandTrigger != cmdArgs[0] {
		return &model.CommandResponse{
			ResponseType: model.CommandResponseTypeEphemeral,
			Text:         fmt.Sprintf("Unknown command: %s", args.Command),
		}, nil
	}

	if len(cmdArgs) < 2 {
		return &model.CommandResponse{
			ResponseType: model.CommandResponseTypeEphemeral,
			Text:         "Please, specify status",
		}, nil
	}

	rKey := REACTION_OFF_KEY + args.UserId
	status := strings.ToLower(cmdArgs[1])
	if status == "off" {
		if err := p.API.KVSet(rKey, []byte("1")); err != nil {
			return nil, model.NewAppError("ExecuteCommand", "plugin.command.execute_command.app_error", nil, err.Error(), http.StatusInternalServerError)
		}
	} else if (status == "on") {
		if err := p.API.KVDelete(rKey); err != nil {
			return nil, model.NewAppError("ExecuteCommand", "plugin.command.execute_command.app_error", nil, err.Error(), http.StatusInternalServerError)
		}
	} else {
		return &model.CommandResponse{
			ResponseType: model.CommandResponseTypeEphemeral,
			Text:         "Please, specify on to enable notifications or off to disable",
		}, nil
	}

	return &model.CommandResponse{
		Text: "The Reactions is turned " + status,
	}, nil
}