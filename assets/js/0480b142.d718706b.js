"use strict";(self.webpackChunkjuno_docs=self.webpackChunkjuno_docs||[]).push([[8070],{8614:(e,n,r)=>{r.r(n),r.d(n,{assets:()=>c,contentTitle:()=>i,default:()=>h,frontMatter:()=>t,metadata:()=>a,toc:()=>d});var s=r(4848),o=r(8453);const t={title:"Frequently Asked Questions"},i="Frequently Asked Questions :question:",a={id:"faq",title:"Frequently Asked Questions",description:"What is Juno?",source:"@site/docs/faq.md",sourceDirName:".",slug:"/faq",permalink:"/next/faq",draft:!1,unlisted:!1,tags:[],version:"current",frontMatter:{title:"Frequently Asked Questions"},sidebar:"main",previous:{title:"Database Snapshots",permalink:"/next/snapshots"}},c={},d=[];function u(e){const n={a:"a",code:"code",h1:"h1",p:"p",pre:"pre",strong:"strong",...(0,o.R)(),...e.components},{Details:r}=n;return r||function(e,n){throw new Error("Expected "+(n?"component":"object")+" `"+e+"` to be defined: you likely forgot to import, pass, or provide it.")}("Details",!0),(0,s.jsxs)(s.Fragment,{children:[(0,s.jsxs)(n.h1,{id:"frequently-asked-questions-question",children:["Frequently Asked Questions ","\u2753"]}),"\n",(0,s.jsxs)(r,{children:[(0,s.jsx)("summary",{children:"What is Juno?"}),(0,s.jsx)(n.p,{children:"Juno is a Go implementation of a Starknet full-node client created by Nethermind to allow node operators to easily and reliably support the network and advance its decentralisation goals. Juno supports various node setups, from casual to production-grade indexers."})]}),"\n",(0,s.jsxs)(r,{children:[(0,s.jsx)("summary",{children:"How can I run Juno?"}),(0,s.jsxs)(n.p,{children:["Check out the ",(0,s.jsx)(n.a,{href:"running-juno",children:"Running Juno"})," guide to learn the simplest and fastest ways to run a Juno node. You can also check the ",(0,s.jsx)(n.a,{href:"running-on-gcp",children:"Running Juno on GCP"})," guide to learn how to run Juno on GCP."]})]}),"\n",(0,s.jsxs)(r,{children:[(0,s.jsx)("summary",{children:"What are the hardware requirements for running Juno?"}),(0,s.jsxs)(n.p,{children:["We recommend running Juno with at least 4GB of RAM and 250GB of SSD storage. Check out the ",(0,s.jsx)(n.a,{href:"hardware-requirements",children:"Hardware Requirements"})," for more information."]})]}),"\n",(0,s.jsxs)(r,{children:[(0,s.jsx)("summary",{children:"How can I configure my Juno node?"}),(0,s.jsxs)(n.p,{children:["You can configure Juno using ",(0,s.jsx)(n.a,{href:"configuring#command-line-params",children:"command line parameters"}),", ",(0,s.jsx)(n.a,{href:"configuring#environment-variables",children:"environment variables"}),", and a ",(0,s.jsx)(n.a,{href:"configuring#configuration-file",children:"YAML configuration file"}),". Check out the ",(0,s.jsx)(n.a,{href:"configuring",children:"Configuring Juno"})," guide to learn their usage and precedence."]})]}),"\n",(0,s.jsxs)(r,{children:[(0,s.jsx)("summary",{children:"How can I update my Juno node?"}),(0,s.jsxs)(n.p,{children:["Check out the ",(0,s.jsx)(n.a,{href:"updating",children:"Updating Juno"})," guide for instructions on updating your node to the latest version."]})]}),"\n",(0,s.jsxs)(r,{children:[(0,s.jsx)("summary",{children:"How can I interact with my Juno node?"}),(0,s.jsxs)(n.p,{children:["You can interact with a running Juno node using the ",(0,s.jsx)(n.a,{href:"json-rpc",children:"JSON-RPC"})," and ",(0,s.jsx)(n.a,{href:"websocket",children:"WebSocket"})," interfaces."]})]}),"\n",(0,s.jsxs)(r,{children:[(0,s.jsx)("summary",{children:"How can I monitor my Juno node?"}),(0,s.jsxs)(n.p,{children:["Juno captures metrics data using ",(0,s.jsx)(n.a,{href:"https://prometheus.io",children:"Prometheus"}),", and you can visualise them using ",(0,s.jsx)(n.a,{href:"https://grafana.com",children:"Grafana"}),". Check out the ",(0,s.jsx)(n.a,{href:"monitoring",children:"Monitoring Juno"})," guide to get started."]})]}),"\n",(0,s.jsxs)(r,{children:[(0,s.jsx)("summary",{children:"Do node operators receive any rewards, or is participation solely to support the network?"}),(0,s.jsx)(n.p,{children:"Presently, running a node does not come with direct rewards; its primary purpose is contributing to the network's functionality and stability. However, operating a node provides valuable educational benefits and deepens your knowledge of the network's operation."})]}),"\n",(0,s.jsxs)(r,{children:[(0,s.jsx)("summary",{children:"How can I view Juno logs from Docker?"}),(0,s.jsx)(n.p,{children:"You can view logs from the Docker container using the following command:"}),(0,s.jsx)(n.pre,{children:(0,s.jsx)(n.code,{className:"language-bash",children:"docker logs -f juno\n"})})]}),"\n",(0,s.jsxs)(r,{children:[(0,s.jsx)("summary",{children:"How can I get real-time updates of new blocks?"}),(0,s.jsxs)(n.p,{children:["The ",(0,s.jsx)(n.a,{href:"websocket#subscribe-to-newly-created-blocks",children:"WebSocket"})," interface provides a ",(0,s.jsx)(n.code,{children:"juno_subscribeNewHeads"})," method that emits an event when new blocks are added to the blockchain."]})]}),"\n",(0,s.jsxs)(r,{children:[(0,s.jsx)("summary",{children:"Does Juno provide snapshots to sync with Starknet quickly?"}),(0,s.jsxs)(n.p,{children:["Yes, Juno provides snapshots for both the Starknet Mainnet and Sepolia networks. Check out the ",(0,s.jsx)(n.a,{href:"snapshots",children:"Database Snapshots"})," guide to get started."]})]}),"\n",(0,s.jsxs)(r,{children:[(0,s.jsx)("summary",{children:"How can I contribute to Juno?"}),(0,s.jsxs)(n.p,{children:["You can contribute to Juno by running a node, starring on GitHub, reporting bugs, and suggesting new features. Check out the ",(0,s.jsx)(n.a,{href:"/#contributions-and-partnerships",children:"Contributions and Partnerships"})," page for more information."]})]}),"\n",(0,s.jsxs)(r,{children:[(0,s.jsxs)("summary",{children:["I noticed a warning in my logs saying: ",(0,s.jsx)(n.strong,{children:'Failed\xa0storing\xa0Block {"err": "unsupported block version"}'}),". How should I proceed?"]}),(0,s.jsxs)(n.p,{children:["You can fix this problem by ",(0,s.jsx)(n.a,{href:"updating",children:"updating to the latest version"})," of Juno.\xa0Check for updates and install them to maintain compatibility with the latest block versions."]})]}),"\n",(0,s.jsxs)(r,{children:[(0,s.jsxs)("summary",{children:["After updating Juno, I receive ",(0,s.jsx)(n.strong,{children:"error while migrating DB."})," How should I proceed?"]}),(0,s.jsx)(n.p,{children:"This error suggests your database is corrupted, likely due to the node being interrupted during migration. This can occur if there are insufficient system resources, such as RAM, to finish the process. The only solution is to resynchronise the node from the beginning. To avoid this issue in the future, ensure your system has adequate resources and that the node remains uninterrupted during upgrades."})]}),"\n",(0,s.jsxs)(r,{children:[(0,s.jsxs)("summary",{children:["I receive ",(0,s.jsx)(n.strong,{children:"Error: unable to verify latest block hash; are the database and --network option compatible?"})," while running Juno. How should I proceed?"]}),(0,s.jsxs)(n.p,{children:["To resolve this issue, ensure that the ",(0,s.jsx)(n.code,{children:"eth-node"})," configuration aligns with the ",(0,s.jsx)(n.code,{children:"network"})," option for the Starknet network."]})]}),"\n",(0,s.jsxs)(r,{children:[(0,s.jsxs)("summary",{children:["I receive ",(0,s.jsx)(n.strong,{children:"process <PID> killed"})," and ",(0,s.jsx)(n.strong,{children:"./build/juno: invalid signature (code or signature have been modified)"})," while running the binary on macOS. How should I proceed?"]}),(0,s.jsx)(n.p,{children:"You need to re-sign the binary to resolve this issue using the following command:"}),(0,s.jsx)(n.pre,{children:(0,s.jsx)(n.code,{className:"language-bash",children:"codesign --sign - ./build/juno\n"})})]})]})}function h(e={}){const{wrapper:n}={...(0,o.R)(),...e.components};return n?(0,s.jsx)(n,{...e,children:(0,s.jsx)(u,{...e})}):u(e)}},8453:(e,n,r)=>{r.d(n,{R:()=>i,x:()=>a});var s=r(6540);const o={},t=s.createContext(o);function i(e){const n=s.useContext(t);return s.useMemo((function(){return"function"==typeof e?e(n):{...n,...e}}),[n,e])}function a(e){let n;return n=e.disableParentContext?"function"==typeof e.components?e.components(o):e.components||o:i(e.components),s.createElement(t.Provider,{value:n},e.children)}}}]);