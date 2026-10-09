
import datetime,json,os,resource,runpy,sys,time,signal
sampler,cost=sys.argv[1:3]
sys.argv=[sampler,*sys.argv[3:]]
started=time.monotonic()
status='completed'
exit_code=0
received_signal=None
def stop(signum,_frame):
    global received_signal
    received_signal=signum
    raise SystemExit(128+signum)
for signum in (signal.SIGTERM,signal.SIGINT):signal.signal(signum,stop)
try:
    runpy.run_path(sampler,run_name='__main__')
except SystemExit as error:
    exit_code=error.code if isinstance(error.code,int) else (0 if error.code is None else 1)
    status='completed' if exit_code==0 else 'failed'
    raise
except BaseException:
    status='failed'
    exit_code=1
    raise
finally:
    own=resource.getrusage(resource.RUSAGE_SELF)
    duration=time.monotonic()-started
    with open('/proc/self/stat') as status_file:
        start_ticks=int(status_file.read().rsplit(') ',1)[1].split()[19])
    clock_ticks=os.sysconf('SC_CLK_TCK')
    process_duration=time.clock_gettime(time.CLOCK_BOOTTIME)-start_ticks/clock_ticks
    receipt={'version':1,'status':status,'exitCode':exit_code,'pid':os.getpid(),'signalNumber':received_signal,
      'finishedAt':datetime.datetime.now(datetime.timezone.utc).isoformat(),
      'scope':'Observer interpreter RUSAGE_SELF, including interpreter imports and launcher; excludes spawned Docker CLI/helpers and all Docker daemon work.',
      'wallSeconds':process_duration,'launcherWallSeconds':duration,'processStartClockTicks':start_ticks,'clockTicksPerSecond':clock_ticks,'userCPUSeconds':own.ru_utime,'systemCPUSeconds':own.ru_stime,
      'cpuSecondsPerWallSecond':(own.ru_utime+own.ru_stime)/process_duration if process_duration>0 else None,
      'maxRSSBytes':own.ru_maxrss*1024,'inputBlockOperations':own.ru_inblock,'outputBlockOperations':own.ru_oublock,
      'voluntaryContextSwitches':own.ru_nvcsw,'involuntaryContextSwitches':own.ru_nivcsw,
      'units':'Linux ru_maxrss KiB converted to bytes; block counts are I/O operations, not bytes. Lifetime wall uses CLOCK_BOOTTIME minus /proc creation ticks, with tick-resolution uncertainty.'}
    descriptor=os.open(cost,os.O_WRONLY|os.O_CREAT|os.O_EXCL,0o600)
    with os.fdopen(descriptor,'w') as output:
        json.dump(receipt,output,indent=2);output.write('\n')
