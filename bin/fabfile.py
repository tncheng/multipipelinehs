import subprocess
from multiprocessing.dummy import Process
from fabric import task
from fabric import Connection
from fabric import ThreadingGroup,SerialGroup
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
    utils.GenerateIPTable({},"local",int(N))
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


def _run(host,user,id,work_dir,start,timeout,alg,key_path):
    conn=Connection(host=host,user=user,connect_kwargs=key_path)
    deadline=150
    with conn.cd(work_dir):
        while 1:
            time.sleep(0.00001)
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
    print('instance {} over'.format(id))
    conn.close()

def _prepare(dir,result_dir,int_ips,N):
    if not os.path.exists(dir):
        os.mkdir(dir)
    if not os.path.exists(result_dir):
        os.mkdir(result_dir)
    run_name=f'run{len(os.listdir(result_dir))+1}'
    os.mkdir(f'{result_dir}/{run_name}')
    utils.generate_ip_table(int_ips,'remote',N)
    subprocess.run('go build -o hotstuff ../server',shell=True)
    subprocess.run(f'mv hotstuff ./{dir}',shell=True)
    subprocess.run(f'cp analyze.py ./{dir}/',shell=True)
    utils.update_config(dir)
    subprocess.run(f'cp ips.txt ./{dir}/ips.txt',shell=True)
    return run_name


def _load_instances():
    from aliyun.setting import load_instance_list
    instances=load_instance_list('ecs_instance_list.csv')
    pub_ips=[ins['pub_ip'] for ins in instances]
    int_ips=[ins['int_ip'] for ins in instances]
    return pub_ips,int_ips


@task 
def setup(ctx):
    hosts,_=_load_instances()
    cmd=[
        'yum -y install tmux',
        'pip3 install numpy'
    ]
    try:
        g=ThreadingGroup(*hosts,user="root",connect_kwargs={"key_filename": "pri_key.pem"})
        g.run(' && '.join(cmd),hide=True)
    except Exception as e:
        print(f'setup failed on instances, due to: {e}')
    
    g.close()


@task
def updatec(ctx,key,value):
    with open("update.json",'r') as cf:
        config=json.load(cf)
    config[key]=int(value)
    if isinstance(config[key],int):
        config[key]=int(value)
    elif isinstance(config[key],str):
        config[key]=str(value)
    with open("update.json",'w') as cf:
        json.dump(config,cf,indent=4)


@task
def remote(ctx,N,alg,mode):
    N=int(N)

    user='root'
    des_dir="/root"
    result_dir='result'
    source_dir='deploy'
    key_path={"key_filename": "pri_key.pem"}

    pub_ips,int_ips=_load_instances()
    
    # compile
    print("prepare deploy file")
    run_name=_prepare(source_dir,result_dir,pub_ips,N)
    hosts=pub_ips[:N]

    work_dir=f'{des_dir}/{source_dir}'
    g=ThreadingGroup(*hosts,user=user,connect_kwargs=key_path)
    try:
        if mode=='update':
            g.put(f'{source_dir}/config.json',work_dir)
            time.sleep(1)
        elif mode=='deploy':
            cmd=[
                f'rm -rf {work_dir}',
                f'mkdir {work_dir}'
            ]
            g.run(' ; '.join(cmd),hide=True)
            time.sleep(1)
            for f in os.listdir(source_dir):
                g.put(f'{source_dir}/{f}',work_dir)
            time.sleep(1)
    except Exception as e:
        print(f'prepare failed on instances, due to: {e}')

    # run benchmark
    print("start benchmark")

    try:
        cmd=f'tmux kill-session'
        g.run(cmd,hide=True)
    except Exception as e:
        print('nothing killed on instances')
    g.close()

    start=time.time()
    tm=10
    pss=[]
    for id,ip in enumerate(hosts):
        instance=Process(target=_run,args=(ip,user,id+1,work_dir,start,tm,alg,key_path))
        pss.append(instance)
        instance.start()
    for p in pss:
        p.join()
    print("benchmark finish")

    #collect result
    g=SerialGroup(*hosts,user=user,connect_kwargs=key_path)
    cmd=[
        f'cd {work_dir}',
        f'tmux new -d -s analyze "python3 analyze.py"'
    ]
    try:
        g.run(' && '.join(cmd),hide=True)
        time.sleep(10)
        g.run('tmux kill-session',hide=True)
    except Exception as e:
        print(f'analysis failed on instances, due to: {e}')

    print("collect result")
    results=[]
    config={}
    for ip in hosts:
        conn=Connection(host=ip,user=user,connect_kwargs=key_path)
        with conn.cd(work_dir):
            info=conn.run('ls',hide=True)
            for f in info.stdout.split('\n'):
                if 'result' in f:
                    conn.get(f'{work_dir}/{f}',f'{result_dir}/{run_name}/{f}')
                    with open(f'{result_dir}/{run_name}/{f}','r') as ref:
                        config=json.load(ref)
                        print(config['result']['tps(tx/s)'],config['result']['average latency(ms)'],config['result']['rate'])
                        results.append(config['result'])
                        config.pop('result')
                    os.remove(f'{result_dir}/{run_name}/{f}')
        conn.close()

    config['results']=results
    with open(f'{result_dir}/{run_name}/results.json','w') as rwf:
        json.dump(config,rwf,indent=4)

    
    # clean deploy
    # cmd=f'rm -rf {work_dir}'

    try:
        # g.run(cmd,hide=True)
        g.close()
    except Exception as e:
        print(f'clear failed on instances, due to: {e}')
