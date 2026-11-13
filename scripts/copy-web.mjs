import { cpSync, mkdirSync } from 'node:fs';
mkdirSync('web',{recursive:true});cpSync('ui/dist/client','web',{recursive:true});
