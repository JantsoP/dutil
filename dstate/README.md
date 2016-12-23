#dstate

dstate is an alternative state tracker to the standard one in discordgo.

It's a bit more advanced but offer more features and it's easier to avoid race conditions with.

Differences:

 - Per guild rw mutex
     + So you don't need to lock the whole state if you want to avoid race conditions
 - Optionally keep deleted messages in state (with a flag on them set if deleted)
 - Presence tracking
 - Optionally remove offline members from state (if your're on limited memory)
 - Set a max message age to only keep messages up untill a certain age in the state
