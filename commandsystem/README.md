# commandsystem

Advanced command system you can use for discord bots, it has generated help menus and automatically parses arguments out of definitions

Documentation is very lacking atm because the system itself is very rough, once i clean it up and think it's ready i will make documentation for it

Example:

```go
func main() {
    session, err := discordgo.New(token)
    if err != nil {
        panic(err)
    }

    // Create a new command system, this function will add a handler
    // and set the prfix to "!"
    system := commandsystem.NewSystem(session, "!")

    Addcommands(&commandsystem.Command{
        Name:        "Echo",
        Description: "Make me say stuff ;)",
        RequiredArgs: 1,
        Arguments: []*commandsystem.ArgDef{
            &commandsystem.ArgDef{Name: "what", Type: commandsystem.ArgumentString},
        },
        // Run is called when the command has been parsed without errors
        // The run function returns a reply (string, error, *MessageEmbed or CommandResponse) and an error
        Run: func(ctx context.Context) (interface{}, error) {
            // Arguments are stored in the context
            // and since it RequiredArgs is set to 1, it will always be available
            return commandsystem.CtxArgs(ctx)[0].Str(), nil
        }
    })

    err = session.Open()
    if err != nil {
        log.Fatal("Failed opening websocket connection")
    }

    log.Println("Started example bot :D stop with ctrl-c")
    select {}
}
```

### Argument parsing

Arguments can be seperated by space or quotes, both can be escaped by adding \ before them and \ can be escaped by using \\\\,
If the last arg is a string it will count the rest of the message as 1 arg.

Example:

say we have an `echo` command with 1 arguement
and an `hello` command with 2 arguments

`!echo this is all 1 argument` - would put everything after !echo in 1 arguement because its the last one

`!hello arg1 arg2 still arg 2` - would put the string `arg1` into argument 1 and the rest into argument 2

`!hello "arg 1 in quotes\" with escaped quote" arg2 still arg 2` - would put the string `arg 1 in quotes" with escaped quote` into argument 1 and the rest into argument 2

`!hello arg\ 1 arg2 still arg 2` - would put the string `arg 1` into argument 1 and the rest into argument 2 because the space is escaped so that it dosen't seperate the `arg` and `1`

Might seem complicated but it is fairly standard

### Argument combos

Can be used to have different combinations of agruments possible, DOC on this is TODO see source for info.