import json
import subprocess
import sys
from pathlib import Path
root=Path(sys.argv[1]).resolve()
if not (root/".bookbeam-formal-copy").is_file() or (root/".git").exists():
 raise SystemExit("Source mutation requires a disposable copy created by verify.py")
out=Path(sys.argv[2]).resolve()
out.mkdir(parents=True,exist_ok=True)
mutations=[
 ('pending-backoff','cmd/auth.go','case "authorization_pending":\n\t\treturn 0, nil','case "authorization_pending":\n\t\treturn 5, nil','./cmd','TestFormalAuthBackoffCases'),
 ('report-on-save-failure','cmd/auth.go','return fmt.Errorf("failed to save token to config: %w", err)','return nil','./cmd','TestFormalAuthSaveFailureNoReport'),
 ('persist-override','pkg/config/config.go','candidate.Token = token','candidate.Token = apply(apply(token, s.env.Token), s.flags.Token)','./pkg/config','TestFormalCredentialsLayerAndPermissionsCases'),
 ('insecure-temporary','pkg/config/config.go','return os.CreateTemp(dir, pattern)','file, err := os.CreateTemp(dir, pattern)\n if err == nil { err = file.Chmod(0644) }; return file, err','./pkg/config','TestFormalCredentialsLayerAndPermissionsCases'),
 ('early-memory-commit','pkg/config/config.go','candidate := s.stored','s.stored.Token = token\n candidate := s.stored','./pkg/config','TestFormalCredentialsFailedSaveIsAtomic'),
 ('run-after-resolution-failure','cmd/root.go','if err := a.load(getenv, home, overrides, output.New(jsonOutput, quiet)); err != nil {\n\t\t\t\treturn err\n\t\t\t}','_ = a.load(getenv, home, overrides, output.New(jsonOutput, quiet))','./cmd','TestFormalReadinessResolutionBlocksCommand'),
 ('bypass-authentication','cmd/authentication.go','if requiresAuthentication(cmd) && a.settings.Token() == "" {','if false && requiresAuthentication(cmd) && a.settings.Token() == "" {','./cmd','TestFormalAuthenticationGate'),
 ('ignore-wait-cancellation','cmd/root.go','case <-ctx.Done():\n\t\treturn ctx.Err()','case <-ctx.Done():\n\t\treturn nil','./cmd','TestFormalAuthCancellation'),
 ('skip-pre-save-cancellation','cmd/auth.go','if err := cmd.Context().Err(); err != nil {\n\t\t\t\treturn err\n\t\t\t}','if false { return context.Canceled }','./cmd','TestFormalAuthCancellationBeforeSave'),
 ('retry-unauthorized','cmd/auth.go','if !ok || apiErr.StatusCode == http.StatusUnauthorized {','if !ok || apiErr.StatusCode == http.StatusUnauthorized && false {','./cmd','TestFormalAuthUnauthorizedIsTerminal'),
]
results=[]
for name,file,before,after,pkg,test in mutations:
 path=root/file
 original=path.read_text()
 assert original.count(before)==1,(name,'source drift')
 try:
  path.write_text(original.replace(before,after))
  run=subprocess.run(['go','test',pkg,'-run','^'+test+'$','-count=1'],cwd=root,capture_output=True,text=True)
  (out/(name+'.out')).write_text(run.stdout+run.stderr)
  caught=run.returncode!=0 and '--- FAIL: '+test in run.stdout
  results.append(dict(name=name,caught=caught,exit=run.returncode))
  assert caught,(name,run.stdout,run.stderr)
 finally:path.write_text(original)
run=subprocess.run(['go','test','./cmd','./pkg/config','-run','^TestFormal','-count=1'],cwd=root,capture_output=True,text=True)
(out/'restored-baseline.out').write_text(run.stdout+run.stderr)
assert run.returncode==0,(run.stdout,run.stderr)
(out/'results.json').write_text(json.dumps(results,indent=2)+'\n')
print(json.dumps(results))
