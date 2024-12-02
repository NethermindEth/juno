"use strict";(self.webpackChunkjuno_docs=self.webpackChunkjuno_docs||[]).push([[9800],{4180:(n,e,t)=>{t.r(e),t.d(e,{assets:()=>h,contentTitle:()=>i,default:()=>c,frontMatter:()=>a,metadata:()=>r,toc:()=>d});var s=t(4848),o=t(8453);const a={title:"Database Snapshots"},i="Database Snapshots :camera_flash:",r={id:"snapshots",title:"Database Snapshots",description:"You can download a snapshot of the Juno database to reduce the network syncing time. Only the blocks created after the snapshot will be synced when you run the node.",source:"@site/versioned_docs/version-0.12.4/snapshots.md",sourceDirName:".",slug:"/snapshots",permalink:"/snapshots",draft:!1,unlisted:!1,tags:[],version:"0.12.4",frontMatter:{title:"Database Snapshots"},sidebar:"main",previous:{title:"Monitoring Juno",permalink:"/monitoring"},next:{title:"Frequently Asked Questions",permalink:"/faq"}},h={},d=[{value:"Mainnet",id:"mainnet",level:2},{value:"Sepolia",id:"sepolia",level:2},{value:"Sepolia-Integration",id:"sepolia-integration",level:2},{value:"Getting snapshot sizes",id:"getting-snapshot-sizes",level:2},{value:"Run Juno with a snapshot",id:"run-juno-with-a-snapshot",level:2},{value:"1. Download the snapshot",id:"1-download-the-snapshot",level:3},{value:"2. Prepare a directory",id:"2-prepare-a-directory",level:3},{value:"3. Extract the snapshot",id:"3-extract-the-snapshot",level:3},{value:"4. Run Juno",id:"4-run-juno",level:3}];function l(n){const e={a:"a",admonition:"admonition",code:"code",h1:"h1",h2:"h2",h3:"h3",p:"p",pre:"pre",strong:"strong",table:"table",tbody:"tbody",td:"td",th:"th",thead:"thead",tr:"tr",...(0,o.R)(),...n.components};return(0,s.jsxs)(s.Fragment,{children:[(0,s.jsxs)(e.h1,{id:"database-snapshots-camera_flash",children:["Database Snapshots ","\ud83d\udcf8"]}),"\n",(0,s.jsx)(e.p,{children:"You can download a snapshot of the Juno database to reduce the network syncing time. Only the blocks created after the snapshot will be synced when you run the node."}),"\n",(0,s.jsx)(e.h2,{id:"mainnet",children:"Mainnet"}),"\n",(0,s.jsxs)(e.table,{children:[(0,s.jsx)(e.thead,{children:(0,s.jsxs)(e.tr,{children:[(0,s.jsx)(e.th,{children:"Version"}),(0,s.jsx)(e.th,{children:"Download Link"})]})}),(0,s.jsx)(e.tbody,{children:(0,s.jsxs)(e.tr,{children:[(0,s.jsx)(e.td,{children:(0,s.jsx)(e.strong,{children:">=v0.9.2"})}),(0,s.jsx)(e.td,{children:(0,s.jsx)(e.a,{href:"https://juno-snapshots.nethermind.dev/files/mainnet/latest",children:(0,s.jsx)(e.strong,{children:"juno_mainnet.tar"})})})]})})]}),"\n",(0,s.jsx)(e.h2,{id:"sepolia",children:"Sepolia"}),"\n",(0,s.jsxs)(e.table,{children:[(0,s.jsx)(e.thead,{children:(0,s.jsxs)(e.tr,{children:[(0,s.jsx)(e.th,{children:"Version"}),(0,s.jsx)(e.th,{children:"Download Link"})]})}),(0,s.jsx)(e.tbody,{children:(0,s.jsxs)(e.tr,{children:[(0,s.jsx)(e.td,{children:(0,s.jsx)(e.strong,{children:">=v0.9.2"})}),(0,s.jsx)(e.td,{children:(0,s.jsx)(e.a,{href:"https://juno-snapshots.nethermind.dev/files/sepolia/latest",children:(0,s.jsx)(e.strong,{children:"juno_sepolia.tar"})})})]})})]}),"\n",(0,s.jsx)(e.h2,{id:"sepolia-integration",children:"Sepolia-Integration"}),"\n",(0,s.jsxs)(e.table,{children:[(0,s.jsx)(e.thead,{children:(0,s.jsxs)(e.tr,{children:[(0,s.jsx)(e.th,{children:"Version"}),(0,s.jsx)(e.th,{children:"Download Link"})]})}),(0,s.jsx)(e.tbody,{children:(0,s.jsxs)(e.tr,{children:[(0,s.jsx)(e.td,{children:(0,s.jsx)(e.strong,{children:">=v0.9.2"})}),(0,s.jsx)(e.td,{children:(0,s.jsx)(e.a,{href:"https://juno-snapshots.nethermind.dev/files/sepolia-integration/latest",children:(0,s.jsx)(e.strong,{children:"juno_sepolia_integration.tar"})})})]})})]}),"\n",(0,s.jsx)(e.h2,{id:"getting-snapshot-sizes",children:"Getting snapshot sizes"}),"\n",(0,s.jsx)(e.pre,{children:(0,s.jsx)(e.code,{className:"language-console",children:"$date\nThu  1 Aug 2024 09:49:30 BST\n\n$curl -s -I -L https://juno-snapshots.nethermind.dev/files/mainnet/latest | gawk -v IGNORECASE=1 '/^Content-Length/ { printf \"%.2f GB\\n\", $2/1024/1024/1024 }'\n172.47 GB\n\n$curl -s -I -L https://juno-snapshots.nethermind.dev/files/sepolia/latest | gawk -v IGNORECASE=1 '/^Content-Length/ { printf \"%.2f GB\\n\", $2/1024/1024/1024 }'\n5.67 GB\n\n$curl -s -I -L https://juno-snapshots.nethermind.dev/files/sepolia-integration/latest | gawk -v IGNORECASE=1 '/^Content-Length/ { printf \"%.2f GB\\n\", $2/1024/1024/1024 }'\n2.4 GB\n"})}),"\n",(0,s.jsx)(e.h2,{id:"run-juno-with-a-snapshot",children:"Run Juno with a snapshot"}),"\n",(0,s.jsx)(e.h3,{id:"1-download-the-snapshot",children:"1. Download the snapshot"}),"\n",(0,s.jsx)(e.p,{children:"First, download a snapshot from one of the provided URLs:"}),"\n",(0,s.jsx)(e.pre,{children:(0,s.jsx)(e.code,{className:"language-bash",children:"wget -O juno_mainnet.tar https://juno-snapshots.nethermind.dev/files/mainnet/latest\n"})}),"\n",(0,s.jsx)(e.h3,{id:"2-prepare-a-directory",children:"2. Prepare a directory"}),"\n",(0,s.jsxs)(e.p,{children:["Ensure you have a directory to store the snapshots. We will use the ",(0,s.jsx)(e.code,{children:"$HOME/snapshots"})," directory:"]}),"\n",(0,s.jsx)(e.pre,{children:(0,s.jsx)(e.code,{className:"language-bash",children:"mkdir -p $HOME/snapshots\n"})}),"\n",(0,s.jsx)(e.h3,{id:"3-extract-the-snapshot",children:"3. Extract the snapshot"}),"\n",(0,s.jsxs)(e.p,{children:["Extract the contents of the downloaded ",(0,s.jsx)(e.code,{children:".tar"})," file into the directory:"]}),"\n",(0,s.jsx)(e.pre,{children:(0,s.jsx)(e.code,{className:"language-bash",children:"tar -xvf juno_mainnet.tar -C $HOME/snapshots\n"})}),"\n",(0,s.jsx)(e.h3,{id:"4-run-juno",children:"4. Run Juno"}),"\n",(0,s.jsxs)(e.p,{children:["Run the Docker command to start Juno and provide the path to the snapshot using the ",(0,s.jsx)(e.code,{children:"db-path"})," option:"]}),"\n",(0,s.jsx)(e.pre,{children:(0,s.jsx)(e.code,{className:"language-bash",children:"docker run -d \\\n  --name juno \\\n  -p 6060:6060 \\\n  -v $HOME/snapshots/juno_mainnet:/snapshots/juno_mainnet \\\n  nethermind/juno \\\n  --http \\\n  --http-port 6060 \\\n  --http-host 0.0.0.0 \\\n  --db-path /snapshots/juno_mainnet \\\n  --eth-node <YOUR ETH NODE>\n"})}),"\n",(0,s.jsx)(e.admonition,{type:"info",children:(0,s.jsxs)(e.p,{children:["Replace <YOUR ETH NODE> with the WebSocket endpoint of your Ethereum node. For Infura users, your address should be: ",(0,s.jsx)(e.code,{children:"wss://mainnet.infura.io/ws/v3/your-infura-project-id"}),". Ensure you use the WebSocket URL (",(0,s.jsx)(e.code,{children:"ws"}),"/",(0,s.jsx)(e.code,{children:"wss"}),") instead of the HTTP URL (",(0,s.jsx)(e.code,{children:"http"}),"/",(0,s.jsx)(e.code,{children:"https"}),")."]})})]})}function c(n={}){const{wrapper:e}={...(0,o.R)(),...n.components};return e?(0,s.jsx)(e,{...n,children:(0,s.jsx)(l,{...n})}):l(n)}},8453:(n,e,t)=>{t.d(e,{R:()=>i,x:()=>r});var s=t(6540);const o={},a=s.createContext(o);function i(n){const e=s.useContext(a);return s.useMemo((function(){return"function"==typeof n?n(e):{...e,...n}}),[e,n])}function r(n){let e;return e=n.disableParentContext?"function"==typeof n.components?n.components(o):n.components||o:i(n.components),s.createElement(a.Provider,{value:e},n.children)}}}]);