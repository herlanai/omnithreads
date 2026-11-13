import {readdirSync,readFileSync,writeFileSync,mkdirSync,existsSync} from 'node:fs';
import {join} from 'node:path';
import {execFileSync} from 'node:child_process';
const out=process.argv[2];mkdirSync(out,{recursive:true});
function collect(dir,name){if(!existsSync(dir))return;for(const file of readdirSync(dir)){if(/^(licen[cs]e|copying|notice)(\.|$)/i.test(file)){try{writeFileSync(join(out,name.replaceAll('/','_')+'-'+file),readFileSync(join(dir,file)))}catch{}}}}
const lock=JSON.parse(readFileSync('ui/package-lock.json','utf8'));
for(const name of Object.keys(lock.packages)){if(name)collect(join('ui',name),name.replace('node_modules/',''))}
const mods=execFileSync('go',['list','-m','-f','{{.Path}}|{{.Dir}}','all'],{encoding:'utf8'}).trim().split('\n');
for(const mod of mods){const [name,dir]=mod.split('|');if(dir)collect(dir,name)}
