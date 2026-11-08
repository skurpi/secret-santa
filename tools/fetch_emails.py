"""Download every Gmail message matching a search into emails/ as .eml files.

Read-only: the mailbox is opened read-only and messages are fetched with
BODY.PEEK, so nothing is marked read, moved or changed. Uses USERNAME and
PASSWORD (the gmail App Password) from config.yml. Standard library only.

    docker run --rm -v "${PWD}:/work" -w /work python:3.13-alpine python tools/fetch_emails.py

Pass a different Gmail search as the first argument if needed, e.g.
    ... python tools/fetch_emails.py 'subject:"Hanukkah person" before:2018/01/01'
"""
import collections
import email
import email.utils
import imaplib
import os
import re
import sys
from email.header import decode_header, make_header

QUERY = sys.argv[1] if len(sys.argv) > 1 else 'subject:"Hanukkah person is"'
OUT = "emails"


def config_value(key):
    with open("config.yml", encoding="utf-8") as f:
        for line in f:
            m = re.match(rf"^{key}:\s*(.*?)\s*$", line)
            if m:
                value = m.group(1)
                if len(value) >= 2 and value[0] == value[-1] and value[0] in "'\"":
                    value = value[1:-1]
                return value
    sys.exit(f"{key} not found in config.yml")


def all_mail_folder(imap):
    # Its name depends on Gmail's display language, so find it by its \All flag.
    _, folders = imap.list()
    for line in folders:
        m = re.match(rb'\((?P<flags>[^)]*)\) "(?P<sep>[^"]*)" (?P<name>.+)$', line)
        if m and rb"\All" in m["flags"]:
            return m["name"].decode()
    sys.exit("Couldn't find Gmail's All Mail folder")


def safe(text):
    return re.sub(r"[^\w .,-]+", "_", text).strip()[:80]


def main():
    imap = imaplib.IMAP4_SSL("imap.gmail.com")
    try:
        imap.login(config_value("USERNAME"), config_value("PASSWORD"))
    except imaplib.IMAP4.error as e:
        sys.exit(f"Gmail login failed: {e}")

    folder = all_mail_folder(imap)
    imap.select(folder, readonly=True)
    quoted = '"' + QUERY.replace("\\", "\\\\").replace('"', '\\"') + '"'
    _, data = imap.uid("SEARCH", "X-GM-RAW", quoted)
    uids = data[0].split()
    print(f"{len(uids)} messages match {QUERY} in {folder}")

    os.makedirs(OUT, exist_ok=True)
    per_year = collections.Counter()
    for uid in uids:
        _, fetched = imap.uid("FETCH", uid, "(BODY.PEEK[])")
        raw = fetched[0][1]
        msg = email.message_from_bytes(raw)
        sent = email.utils.parsedate_to_datetime(msg["Date"])
        subject = str(make_header(decode_header(msg.get("Subject", ""))))
        name = f"{sent:%Y-%m-%d_%H%M%S}_{safe(subject)}_{uid.decode()}.eml"
        with open(os.path.join(OUT, name), "wb") as f:
            f.write(raw)
        per_year[sent.year] += 1

    imap.logout()
    for year in sorted(per_year):
        print(f"  {year}: {per_year[year]} emails")
    print(f"Saved to {OUT}/ (spoilers, gitignored)")


if __name__ == "__main__":
    main()
