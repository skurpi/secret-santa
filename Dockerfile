FROM python:2-alpine

COPY requirements.txt /
RUN pip install --no-cache-dir -r requirements.txt

COPY secret_santa.py config.yml /santa/

ENTRYPOINT ["/santa/secret_santa.py"]
