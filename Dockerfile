FROM continuumio/miniconda3:25.3.1-1

# Set the working directory for the application
WORKDIR /app

# 1. Copy and create the conda env
COPY environment.yml .
RUN conda env create -f environment.yml

# 2. Activate the env for BUILD-TIME commands
SHELL ["conda", "run", "-n", "venv", "/bin/bash", "-c"]

# 3. Copy and install pip requirements
COPY requirements.txt .
RUN pip install -r requirements.txt

# 4. Copy the rest of the application code
# (This copies the 'server' folder into /app/server)
COPY . .

EXPOSE 5000

# 5. Correct CMD
# Runs 'python server/server.py' from the /app directory
CMD ["/opt/conda/envs/venv/bin/python", "server/server.py"]