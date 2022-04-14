
import time
from numpy import nan_to_num
from numpy import mean
import json
import os

from yaml import load


def load_log(work_dir):
    txs=0
    logf=''
    for f in os.listdir(work_dir):
        if '.log' in f:
            logf=f
    fp=open(f'{work_dir}/{logf}')
    timelist=[]
    latencys=[]
    blocks=0
    stat={}
    alg=""
    id=''
    with open('ips.txt','r') as f:
        N=len(f.readlines())
    for line in iter(fp):
        if "consensus" in line:
            id=line.split()[-3]
            alg=line.split(':')[-1]
        if "committed" in line:
            a=line.split(' ')[1]+'-'+line.split(' ')[2].split('.')[0]
            st=time.mktime(time.strptime(a,"%Y/%m/%d-%H:%M:%S"))
            timelist.append(st)
            commitedtx=line.split(':')[5][:-6]
            txs+=int(commitedtx)
            if int(commitedtx)==0:
                continue
            blocks+=1
        if "latency" in line:
            latencys.append(nan_to_num(float(line.split(' ')[-2])))
    if len(timelist)<1:
        print("there is no valid log")
        return None
    duration=timelist[-1]-timelist[0]
    stat['id']=id
    stat['algorithm']=alg[:-1]
    stat['running time(s)']=duration
    stat['txs']=txs
    stat['tps(tx/s)']=int(txs/duration)
    stat['average latency(ms)']=mean(latencys)
    stat['replicas']=N
    print(json.dumps(stat,indent=2))
    stat['latencys']=str(latencys)
    with open(f'{work_dir}/config.json','r') as cf:
        config=json.load(cf)
    config['result']=stat
    with open(f'{work_dir}/result_{id}.json','w') as wf:
        json.dump(config,wf,indent=4)

load_log('./')
    