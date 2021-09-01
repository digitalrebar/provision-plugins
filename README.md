
# provision-plugins

These repo contains the ipmi, slack, packet-ipmi, and virtualbox-ipmi plugins.

Also some docs on building plugins.

## Overview

A plugin provides a plugin-provider object to the system.  This makes the program as launchable.
When the admin creates a plugin using the provider, the program is launched with specific configuration
to provide an instance based upon that config.

A plugin-provider must provide the following things:

1. Provide a `define` cli method that returns the plugin-provider model object as a JSON blob.
2. Provide a `listen` cli method that takes an inbound and outbound UNIX domain socket paths as additional args.
3. Provide an optional `unpack` cli method that takes a path to put files in.

The `define` cli method is not long running and returns.

The `unpack` cli method is not long running and returns the results of the unpack operation.

The `listen` cli method is expected to continue until stopped.  The in-bound socket used to
provide the DRP -> Plugin API calls, Config, Stop, Action, and Publish.

These are handled by these endpoints that are not exposed on the API port:

* /api-plugin/v4/config  - POST - map[string]interface{}
* /api-plugin/v4/stop    - POST - no data
* /api-plugin/v4/action  - POST - models.Action
* /api-plugin/v4/publish - POST - models.Event

Additionally, all requests on the DRP API port will be forwarded to the plugin if sent to:

* /plugin-apis/:plugin/\* - ANY - All Data

Where :plugin is the name of the plugin in the plugin object space.  No translation of URL
will occur.  It is a direct mapping of the two spaces.

The DRP Endpoint is running a REST API server on the out-bound socket for the plugin
to send information back.

The current endpoints for that API are:

* /api-server-plugin/v4/log     - POST - Logger.Line
* /api-server-plugin/v4/publish - POST - Model.Event

All the basic input and output options are provided in a plugin wrapper.

## Plugin Wrapper

This is set of boilerplate code that provides GO functions and interfaces to support easier construction of a plugin.
The wrapper is based on a Cobra App and allows for command line inspection of the plugin.

This resides in `github.com/digitalrebar/provision/plugin`

An example is in `provision/cmds/incrementer/incrementer.go`

The following is the main function need to initial and run the plugin.

`def` is the plugin-provider model structure.
`&Plugin{}` implements the interfaces that this plugin wants to handle.

Example main and imports

```
package main

//go:generate sh -c "cd content ; drpcli contents bundle ../content.go"

import (
        "fmt"
        "os"

        "github.com/VictorLowther/jsonpatch2/utils"
        "github.com/digitalrebar/logger"
        "github.com/digitalrebar/provision"
        "github.com/digitalrebar/provision/api"
        "github.com/digitalrebar/provision/models"
        "github.com/digitalrebar/provision/plugin"
)

func main() {
	plugin.InitApp("incrementer", "Increments a parameter on a machine", version, &def, &Plugin{})
	err := plugin.App.Execute()
	if err != nil {
		os.Exit(1)
	}
}
```

## Define Struct

The plugin-provider structure informs that DRP endpoint about the plugin-provider.  This is provided on
the `InitApp` call and is output as part of the `define` cli method.

It looks like this:

```
        def     = models.PluginProvider{
                Name:          "incrementer",
                Version:       version,
                PluginVersion: 4,
                HasPublish:    true,
                AvailableActions: []models.AvailableAction{
                        models.AvailableAction{Command: "increment",
                                Model:          "machines",
                                OptionalParams: []string{"incrementer/step", "incrementer/parameter"},
                        },
                        models.AvailableAction{Command: "reset_count",
                                Model:          "machines",
                                RequiredParams: []string{"incrementer/touched"},
                        },
                        models.AvailableAction{Command: "incrstatus"},
                },
                Content: contentYamlString,
        }
```

It provides the name and version of the plugin.  It also indicates the plugin version it is.  The structure indicates whether it wants
event streams based upon the `HasPublish` flag.  It also contains a list of actions that it wants to perform.  The actions define
the object type they are available for and the required parameters needed.  If no Model is provided, it is assumed to be a system-level
action.

The final item is a content Yaml string that is passed to the content system when the plugin-provider is loaded.

## Content Layer

The content layer can be built by the `drpcli` command.  By specifying a \.go file extension as an output file, `drpcli` will generate
a go file with a global string named `contentYamlString`.

A `go generate` line can be used to assist with the construction assuming `drpcli` is in the path.

```
//go:generate sh -c "cd content ; drpcli contents bundle ../content.go"
```

This will generate `content.go` file from the `content` directory where the main go file is located.  The `content` directory
should be in directory file store format.

## Documentation for the Plugin

Documentation can be automatically generated by the plugin build process and made available for the doc system
in `provision` by including the following generate commands in your main go file.

```cassandraql
//go:generate sh -c "cd content ; drpcli contents bundle ../content.yaml"
//go:generate sh -c "drpcli contents document content.yaml > PLUGIN_NAME_HERE.rst"
//go:generate rm content.yaml
```

NOTE: Remember to change `PLUGIN_NAME_HERE` to the plugin's name.

This will use the pieces of the content package and generate an RST file that will be published to AWS on
a completed build.  This file can be added to the main docs by editing the `conf.py` file at
`https://github.com/digitalrebar/provision`.

## Config Action

The Config Action is called by the DRP Endpoint when a plugin is created that uses this plugin-provider.

The Plugin struct passed on `InitApp` must implement the `PluginConfig` interface.  This provides the following function:

* Config(logger.Logger, \*api.Client, map[string]interface{}) \*models.Error

The routine is passed a logger to use for logging.  The results will be sent back to the DRP Endpoint.
The routine is also passed a DRP API Client with authentication already setup for using a plugin-specific token with
full access to the system.  The last parameter is a `map[string]interface{}`.  These are the parameters specified on
the plugin object.  The `RequiredParameters` is enforced by the DRP Endpoint.

The plugin should take actions to validate that it can operate, setup additional components, initialize state, and store the
api client session if desired.  Any error returned will be cause the plugin to fail to be created and errors required on the
plugin object.

## Action Action

The Action Action is called by the DRP Endpoint through the plugin wrapper when a object's action is called.  The DRP
Endpoint will validate the `RequiredParameters` and fill in the `OptionalParameters`.  For the actions to be handled,
the Plugin struct passed to `InitApp` must implement the `PluginActor` interface.  This provides the following function:

* Action(logger.Logger, \*models.Action) (interface{}, \*models.Error)

The routine receives a logger to output log information to the DRP endpoint.  The Action requested is defined by the
models.Action object.  This will contain an object reference, command requested, and parameters.  The routine
should return an object that will be returned to the DRP API caller as JSON or an Error object that will be sent
to the user.  The API return code can be specified here for errors.

In general, this function will be a big switch to handle the various call types.

## Publish Action

The Publish Action is called by the DRP Endpoint through the plugin wrapper when an event is published.  The
DRP Enpoint will send the event structure.  To handle events, the plugin must mark the `HasPublish` flag as true
in the plugin-provider structure and implement the `PluginPublisher` interface.  This provide the following function:

* Publish(logger.Logger, \*models.Event) \*models.Error

The routine recieves a logger to log messages and the Event that is being published.  The routine may do
what it wishes with the Event and returns `nil` or an error.   The Event is not published under a lock and
may call back into the API.

## Unpack Action

The Unpack Action is called by the `unpack` CLI method provided by the plugin wrapper.  The DRP Endpoint
calls this CLI method after `define` during plugin discovery.  It is not part of the Plugin's operation or
tied to a specific Plugin instance.  The interface function is:

* Unpack(logger.Logger, string) error

The routine receives a logger to log messages and a path.  The plugin-provider places files into the path.

## Calls to DRP

The plugin wrapper provides a mechanism to publish events as well.  `plugin.Publish` can be used to send events as well.

## Building a Plugin

At a minimum, you need to do this:

* go generate myplugin/myplugin.go
* go build -o myplugin myplugin/\*.go

If the plugin resides in this repos, `tools/build.sh` should be updated to build the plugin and versions will automatically be updated and injected.




.. Release v4.8.0 Start
