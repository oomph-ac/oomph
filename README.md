# Oomph
Oomph is a military grade interception based anti-cheat proxy for Minecraft: Bedrock Edition.

## How does Oomph work?
Oomph acts as an intermediary between your server and Minecraft clients, processing all packets sent by the client and the server. It performs various checks on all packets to detect potential cheats. This versatility lets Oomph function on different server software with ease.

Oomph strives to be highly performant per-player, while also being able to detect a variety of cheats and exploits.

## Configuration
At the moment, Oomph is undergoing a refactor, and the configuration is not yet complete for Oomph.

## Credits
* [ethaniccc](https://www.github.com/ethaniccc) - Created the systems for validating combat and movement, while keeping lag-compensation in mind.
* [JustTalDevelops](https://github.com/JustTalDevelops) - Created the base of Oomph, making it able to intercept packets, and avoiding pesky import cycles.

