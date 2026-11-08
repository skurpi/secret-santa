# secret-santa

Draws secret santas and emails everyone who they're buying for.

- Couples never get each other.
- Nobody gets someone they've had in the last few years.
- The organiser can take part too: you never see who got whom.

All you need is Docker.

## First time

    cp config.yml.template config.yml

Fill in `config.yml`; each setting is explained in the file. For Gmail you
need an [App Password](https://myaccount.google.com/apppasswords).

## Every year

Update `config.yml` (who's in, the message), then try it out. This shows a
sample draw and sends nothing:

    docker compose run --rm --build santa

When it looks right, send for real:

    docker compose run --rm --build santa --send

That's it. Each draw is saved in `history/`, so next year's draw can avoid
repeats. Don't open those files unless you want spoilers.

If sending stops partway, it tells you who already got an email. Run it again
and tell those people to ignore the first one.

## Extras

Run the tests:

    docker compose run --rm test

Download old draw emails from Gmail into `emails/` (read-only, spoilers):

    docker run --rm -v "${PWD}:/work" -w /work python:3.13-alpine python tools/fetch_emails.py
