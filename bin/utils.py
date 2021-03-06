
import time

import json

from numpy import nan_to_num
from numpy import mean

def update_config(dir):
    with open("update.json",'r') as uc:
        config=json.load(uc)
    with open("config.json",'r') as oc:
        origin_config=json.load(oc)
    
    for k,v in config.items():
        origin_config[k]=v
    with open("{}/config.json".format(dir),'w') as wf:
        json.dump(origin_config,wf,indent=4)


def generate_ip_table(int_ips,mode,N):
    if mode=='remote':
        ip_tables=[]
        if N>len(int_ips):
            raise Exception(f'size of instances error, {N},{len(int_ips)}')
        ip_tables=[f'{ip}\n' for ip in int_ips[:N]]
        with open("ips.txt",'w') as wf:
            wf.writelines(ip_tables)
    else:
        with open("ips.txt",'w') as wf:
            wf.writelines([f'127.0.0.1\n' for _ in range(N)])
        



        


def Analyze(config_dir,result_dir,log,N):
    txs=0
    fp=open("{}/{}".format(result_dir, log))
    timelist=[]
    latencys=[]
    blocks=0
    stat={}
    alg=""
    id=''
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
    print(json.dumps( stat,indent=2))
    stat['latencys']=str(latencys)
    return stat