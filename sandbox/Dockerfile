FROM python:3.12-slim
RUN pip install --no-cache-dir ruff==0.14.3
WORKDIR /opt/nexcode
COPY bootstrap.py /opt/nexcode/bootstrap.py
ENV PYTHONDONTWRITEBYTECODE=1 PYTHONIOENCODING=utf-8
USER 65534:65534
ENTRYPOINT ["python", "-I", "-B", "/opt/nexcode/bootstrap.py"]
