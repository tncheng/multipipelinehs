import time

import json

from numpy import nan_to_num

def UpdateConfig(dir):
    with open("update.json",'r') as uc:
        config=json.load(uc)
    with open("config.json",'r') as oc:
        origin_config=json.load(oc)
    
    for k,v in config.items():
        origin_config[k]=v
    with open("{}/config.json".format(dir),'w') as wf:
        json.dump(origin_config,wf,indent=4)


def GenerateIPTable(ip_ports,mode,N):
    if mode=='remote':
        ips=[]
        ports=[]
        phy_nodes=len(ip_ports)
        
        for k,v in reversed(ip_ports.items()):
            ips.append(k)
            ports.append(v)
        
        ip_tables=[]
        nodes=[]
        for i in range(N):
            ip_tables.append(f'{ips[i%(phy_nodes)]}\n')
            nodes.append(ports[i%phy_nodes])
        
        with open("ips.txt",'w') as wf:
            wf.writelines(ip_tables)
        return nodes
    else:
        with open("ips.txt",'w') as wf:
            wf.writelines([f'127.0.0.1\n' for _ in range(N)])
        return None
        



        


def Analyze(config_dir,result_dir,log,N):
    txs=0
    fp=open("{}/{}".format(result_dir, log))
    timelist=[]
    latency=0.0
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
            latency=nan_to_num(float(line.split(' ')[-2]))
    if len(timelist)<1:
        print("there is no valid log")
        return None
    duration=timelist[-1]-timelist[0]
    stat['id']=id
    stat['algorithm']=alg[:-1]
    stat['running time(s)']=duration
    stat['txs']=txs
    stat['tps(tx/s)']=int(txs/duration)
    stat['latency(ms)']=latency
    stat['replicas']=N
    print(json.dumps( stat,indent=2))
    return stat