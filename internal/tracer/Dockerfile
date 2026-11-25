FROM python:3.12-alpine

WORKDIR /app

COPY tracer.py /app/tracer.py
COPY runner.py /app/runner.py

RUN chmod 444 /app/tracer.py /app/runner.py

USER 65534:65534

ENTRYPOINT ["python", "-I", "-B", "-u", "-c", "import sys; sys.path.insert(0, '/app'); from runner import main; main()"]