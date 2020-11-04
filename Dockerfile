FROM ubuntu:18.04

RUN apt-get update
RUN apt install -y ffmpeg

RUN apt-get update
RUN apt-get install -y gcc  --fix-missing
RUN apt-get install -y make
RUN apt-get install -y wget

RUN wget https://dl.google.com/go/go1.15.linux-amd64.tar.gz  
RUN tar -xvf go1.15.linux-amd64.tar.gz  
RUN mv go /usr/local  

RUN apt-get install -y git

ENV PATH /usr/local/go/bin:$PATH