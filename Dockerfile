FROM continuumio/miniconda3:25.3.1-1

WORKDIR /server

# 1. Copy and create the conda env
COPY environment.yml .
RUN conda env create -f environment.yml

# 2. Activate the env for BUILD-TIME commands
SHELL ["conda", "run", "-n", "venv", "/bin/bash", "-c"]

# 3. Copy and install pip requirements
COPY requirements.txt .
RUN pip install -r requirements.txt

COPY . .

EXPOSE 5000

CMD ["/opt/conda/envs/e2ee-env/bin/python", "./server/server.py"]