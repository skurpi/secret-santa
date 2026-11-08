# secret-santa

Randomly assigns secret santas and emails each person who they're buying for.

- Every valid draw is equally likely (the old Python version wasn't fair, see below).
- Couples (or whole households) never draw each other.
- No repeats: each `--send` saves the draw to `history/<year>.yml` (gitignored),
  and later draws avoid the pairs from the last `AVOID_YEARS` years (default 2).
- Any number of people, odd or even. Comment someone out to skip them a year.

The only thing you need installed is Docker.

## Every year

1. Edit `config.yml`: who's in, who's a couple, the message.
   First time? `cp config.yml.template config.yml`.
   For gmail you need an [App Password](https://myaccount.google.com/apppasswords).

2. Dry run: shows a sample draw and an example email. Sends nothing.

       docker compose run --rm --build santa

3. Send. Makes a fresh draw, emails everyone, saves the draw to `history/`.

       docker compose run --rm --build santa --send

`--send` only prints who was emailed, not who they got, so the organiser can take
part too. `history/<year>.yml` holds the answers, so don't open it unless you
want spoilers.

If sending fails partway, it tells you who was already emailed. Run `--send`
again for a fresh draw and tell those people to ignore their first email.

### Trying the email side without bothering the family

Put a config in its own folder (history gets saved next to the config) with a
few participants using your own address, e.g. `you+ann@gmail.com`:

    docker compose run --rm --build santa --config smoke/config.yml --send

## Development

Tests run in Docker too:

    docker compose run --rm test

The image build runs `go vet` and the tests, so a broken build can't send
emails. There's also a dev container for VS Code (`.devcontainer/`).

## Why the rewrite

The original Python 2 script gave each person their recipient in config
order, picking from whoever was left and starting over on a dead end. That
never broke a rule, but it wasn't fair: whoever is near the end of the list
tends to get the leftovers, and because the list order stays the same each
year, the same pairs kept coming up. On the 2026 family list one pairing came
up 26% of the time when a fair draw gives about 13%.

The new draw shuffles everyone and keeps the shuffle only if it breaks no rule,
so every valid draw is equally likely. `TestFamilyDrawIsFair` checks that
against exact counts.
