import subprocess
from multiprocessing.dummy import Process
from fabric import task
from fabric import Connection
from fabric import ThreadingGroup
import utils
import os
import time
import json






@task
def local(ctx,log_level,N,alg):
    N=int(N)
    times=150
    print("local running")
    source_dir="deploy"
    result_dir="result"
    _=utils.GenerateIPTable({},"local",int(N))
    run_name=_prepare(source_dir,result_dir)
    for id in range(len(open(f'{source_dir}/ips.txt','r').readlines())):
        subprocess.run(f'cd {source_dir};./hotstuff -id {id+1} -log_dir=../{result_dir}/{run_name} -log_level={log_level} -algorithm={alg} &',shell=True)
    time.sleep(times)
    _stopl()
    _analaze(source_dir,f'{result_dir}/{run_name}',N)


# stop local process
def _stopl():
    print("stop running")
    subprocess.run('pkill hotstuff',shell=True)


def _analaze(config_dir,result_dir,N):
    with open(f'{config_dir}/config.json','r') as cf:
        currentConfig=json.load(cf)
    stats=[]
    for f in os.listdir(result_dir):
        if 'log' in f:
            stat=utils.Analyze(config_dir,result_dir,f,N)
            stats.append(stat)
    currentConfig['result']=stats
    with open(f'{result_dir}/result.json','w') as wf:
        json.dump(currentConfig,wf,indent=4)

@task
def clear(ctx):
    _clear("deploy")
    _clear('result')


def _clear(dir):
    subprocess.run(f'rm -rf {dir}',shell=True)


def _run(host,user,port,id,work_dir,start,timeout,alg):
    conn=Connection(host=host,user=user,port=port)
    deadline=150
    with conn.cd(work_dir):
        try:
            conn.run('tmux kill-session',hide=True)
        except Exception as e:
            print("nothing killed on {}".format(id))        
        while 1:
            time.sleep(0.0001)
            if time.time()-start>timeout:
                cmd=f'./hotstuff -id {id} -log_dir=./ -log_level=info -algorithm={alg}'
                conn.run(f'tmux new -d -s hotstuff{id} "{cmd}"')
                print(f'running {id}')
                break
    time.sleep(deadline)
    with conn.cd(work_dir):
        try: 
            conn.run('tmux kill-session')
        except Exception as e:
            print("no process to kill")
    print('worker {} over'.format(id))

def _prepare(dir,result_dir):
    if not os.path.exists(dir):
        os.mkdir(dir)
    if not os.path.exists(result_dir):
        os.mkdir(result_dir)
    run_name=f'run{len(os.listdir(result_dir))+1}'
    os.mkdir(f'{result_dir}/{run_name}')
        
    subprocess.run('go build -o hotstuff ../server',shell=True)
    subprocess.run('mv hotstuff ./{}'.format(dir),shell=True)
    utils.UpdateConfig(dir)
    subprocess.run('cp ips.txt ./{}/ips.txt'.format(dir),shell=True)
    return run_name




@task
def remote(ctx,N,alg):
    N=int(N)
    user="chengtaining"
    host_ip="113.55.112.201"
    interner_ports={
        "192.168.1.11":6011,
        "192.168.1.12":6012,
        "192.168.1.13":6013,
        "192.168.1.14":6014,
        "192.168.1.15":6015,
        "192.168.1.16":6016,
        "192.168.1.17":6017,
        "192.168.1.18":6018,
        "192.168.1.19":6019,
        }
    nodes=utils.GenerateIPTable(interner_ports,"remote",N)
    dest_dir=f'/home/{user}'

    source_dir="deploy"
    result_dir="result"
    work_path=dest_dir+'/'+source_dir

    # compile
    print("prepare deploy file")
    run_name=_prepare(source_dir,result_dir)

    # deploy to remote
    servers=[Connection(host=host_ip,user=user,port=p) for p in nodes]
    deployG=ThreadingGroup.from_connections(servers)
    # clean deploy
    cmd='rm -rf {}'.format(work_path)
    deployG.run(cmd,hide=True)

    print("deploy binary")
    cmd="mkdir {}".format(work_path)
    deployG.run(cmd,hide=True)
    
    for f in os.listdir(source_dir):
        deployG.put("{}/{}".format(source_dir,f),work_path)
    deployG.close()

    # run benchmark
    print("start benchmark")
    start=time.time()
    tm=10
    pss=[]
    for id,p in enumerate(nodes):
        instance=Process(target=_run,args=(host_ip,user,p,id+1,work_path,start,tm,alg))
        pss.append(instance)
        instance.start()
    for p in pss:
        p.join()
    print("benchmark finish")

    #collect result
    print("collect result")
    for id,p in enumerate(nodes):
        conn=Connection(host=host_ip,user=user,port=p)
        with conn.cd(work_path):
            info=conn.run('ls',hide=True)
            for log in info.stdout.split('\n'):
                if "log" in log:
                    conn.get("{}/{}".format(work_path,log),"./{}/{}".format(f'{result_dir}/{run_name}',log))
    
    _analaze(source_dir,f'{result_dir}/{run_name}',N)