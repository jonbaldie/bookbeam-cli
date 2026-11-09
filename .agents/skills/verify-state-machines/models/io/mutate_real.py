from pathlib import Path
import subprocess,json
import sys
r=Path(sys.argv[1]).resolve()
if not (r/".bookbeam-formal-copy").is_file() or (r/".git").exists():
 raise SystemExit("Source mutation requires a disposable copy created by verify.py")
out=Path(sys.argv[2]).resolve()
out.mkdir(parents=True,exist_ok=True)
results=[]
cases=[('download-truncates','cmd/files.go','os.CreateTemp(filepath.Dir(destPath), ".bookbeam-download-*")','os.Create(destPath)','./cmd','TestFormalDownloadFailurePreservesDestination'),('transfer-no-timeout','pkg/client/client.go','context.WithTimeout(ctx, c.TransferTimeout)','context.WithCancel(ctx)','./pkg/client','TestFormalTransferDeadlineAndCancelledRequest'),('csv-cancel-publishes','cmd/downloaders.go','if err := ctx.Err(); err != nil {\n\t\treturn 0, err\n\t}','if false {\n\t\treturn 0, context.Canceled\n\t}','./cmd','TestFormalCSVSuccessfulPublicationAndCancellation')]
for name,file,old,new,pkg,test in cases:
 p=r/file;s=p.read_text();assert s.count(old)==1,(name,s.count(old));p.write_text(s.replace(old,new))
 try:
  run=subprocess.run(['go','test',pkg,'-run','^'+test+'$','-count=1'],cwd=r,text=True,stdout=subprocess.PIPE,stderr=subprocess.STDOUT)
  (out/(name+'.out')).write_text(run.stdout)
  caught=run.returncode!=0 and '--- FAIL: '+test in run.stdout
  results.append(dict(name=name,caught=caught,exit=run.returncode))
  assert caught,(name,run.stdout)

 finally:p.write_text(s)

run=subprocess.run(['go','test','./cmd','./pkg/client','./pkg/output','-run','^TestFormal','-count=1'],cwd=r,text=True,stdout=subprocess.PIPE,stderr=subprocess.STDOUT)
(out/'restored-baseline.out').write_text(run.stdout)
assert run.returncode==0,run.stdout
(out/'results.json').write_text(json.dumps(results,indent=2)+'\n')
print(json.dumps(results))
