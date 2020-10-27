FROM ubuntu:18.04

RUN apt-get update
RUN apt-get install -y npm
RUN apt install -y ffmpeg

COPY ./package.json ./packge.json
RUN npm install