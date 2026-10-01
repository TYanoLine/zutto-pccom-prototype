# Host Skeletons

Host skeletons are canonical world-engine facts persisted in PostgreSQL. They are not invented by an LLM at connection time.

## Generation

A world has a random 64-bit seed. Each directory position derives a deterministic skeleton from that seed. A host reset increments that host's `generation`, giving it a fresh skeleton while preserving the directory slot and phone number. A world reset deletes the world row and descendants; the next bootstrap receives a new world seed and catalog.

Current skeleton fields:

- host-program family
- line count
- maximum baud rate
- founding date
- popularity
- member count

## Historical accuracy status

The current distributions for host-program family, line count, founding date, popularity, and member count are **station-specific fictional reconstruction**. They are scaffolding for the persistent-world engine, not claims about measured 1996 Japanese BBS market shares.

The host-program family labels name historically researched families, but their present selection weights are not historically validated. Replace the weights when primary or strong secondary evidence is available. Host-program-specific menus, commands, board models, mail, chat, file transfer, unread tracking, and other behavior remain responsibilities of separate host runtimes; the skeleton must not create a fictional shared host UI.

## HAKATA generator-evaluation exception

Successful reconnects temporarily reset this fixed station's article sample,
allowing repeated quality checks. Other persistent-world data and hosts are not
reset through that station-specific mechanism.
