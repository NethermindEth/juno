"use strict";(self.webpackChunkjuno_docs=self.webpackChunkjuno_docs||[]).push([[1220],{7658:(e,n,s)=>{s.r(n),s.d(n,{assets:()=>c,contentTitle:()=>i,default:()=>h,frontMatter:()=>t,metadata:()=>a,toc:()=>d});var r=s(4848),o=s(8453);const t={title:"Frequently Asked Questions"},i="Frequently Asked Questions :question:",a={id:"faq",title:"Frequently Asked Questions",description:"What is Juno?",source:"@site/versioned_docs/version-0.13.0/faq.md",sourceDirName:".",slug:"/faq",permalink:"/0.13.0/faq",draft:!1,unlisted:!1,tags:[],version:"0.13.0",frontMatter:{title:"Frequently Asked Questions"},sidebar:"main",previous:{title:"Database Snapshots",permalink:"/0.13.0/snapshots"}},c={},d=[];function u(e){const n={a:"a",code:"code",h1:"h1",p:"p",pre:"pre",strong:"strong",...(0,o.R)(),...e.components},{Details:s}=n;return s||function(e,n){throw new Error("Expected "+(n?"component":"object")+" `"+e+"` to be defined: you likely forgot to import, pass, or provide it.")}("Details",!0),(0,r.jsxs)(r.Fragment,{children:[(0,r.jsxs)(n.h1,{id:"frequently-asked-questions-question",children:["Frequently Asked Questions ","\u2753"]}),"\n",(0,r.jsxs)(s,{children:[(0,r.jsx)("summary",{children:"What is Juno?"}),(0,r.jsx)(n.p,{children:"Juno is a Go implementation of a Starknet full-node client created by Nethermind to allow node operators to easily and reliably support the network and advance its decentralisation goals. Juno supports various node setups, from casual to production-grade indexers."})]}),"\n",(0,r.jsxs)(s,{children:[(0,r.jsx)("summary",{children:"How can I run Juno?"}),(0,r.jsxs)(n.p,{children:["Check out the ",(0,r.jsx)(n.a,{href:"running-juno",children:"Running Juno"})," guide to learn the simplest and fastest ways to run a Juno node. You can also check the ",(0,r.jsx)(n.a,{href:"running-on-gcp",children:"Running Juno on GCP"})," guide to learn how to run Juno on GCP."]})]}),"\n",(0,r.jsxs)(s,{children:[(0,r.jsx)("summary",{children:"What are the hardware requirements for running Juno?"}),(0,r.jsxs)(n.p,{children:["Check out the latest system requirements in the ",(0,r.jsx)(n.a,{href:"hardware-requirements",children:"Hardware Requirements"})," section."]})]}),"\n",(0,r.jsxs)(s,{children:[(0,r.jsx)("summary",{children:"How can I configure my Juno node?"}),(0,r.jsxs)(n.p,{children:["You can configure Juno using ",(0,r.jsx)(n.a,{href:"configuring#command-line-params",children:"command line parameters"}),", ",(0,r.jsx)(n.a,{href:"configuring#environment-variables",children:"environment variables"}),", and a ",(0,r.jsx)(n.a,{href:"configuring#configuration-file",children:"YAML configuration file"}),". Check out the ",(0,r.jsx)(n.a,{href:"configuring",children:"Configuring Juno"})," guide to learn their usage and precedence."]})]}),"\n",(0,r.jsxs)(s,{children:[(0,r.jsx)("summary",{children:"How can I update my Juno node?"}),(0,r.jsxs)(n.p,{children:["Check out the ",(0,r.jsx)(n.a,{href:"updating",children:"Updating Juno"})," guide for instructions on updating your node to the latest version."]})]}),"\n",(0,r.jsxs)(s,{children:[(0,r.jsx)("summary",{children:"How can I interact with my Juno node?"}),(0,r.jsxs)(n.p,{children:["You can interact with a running Juno node using the ",(0,r.jsx)(n.a,{href:"json-rpc",children:"JSON-RPC"})," and ",(0,r.jsx)(n.a,{href:"websocket",children:"WebSocket"})," interfaces."]})]}),"\n",(0,r.jsxs)(s,{children:[(0,r.jsx)("summary",{children:"How can I monitor my Juno node?"}),(0,r.jsxs)(n.p,{children:["Juno captures metrics data using ",(0,r.jsx)(n.a,{href:"https://prometheus.io",children:"Prometheus"}),", and you can visualise them using ",(0,r.jsx)(n.a,{href:"https://grafana.com",children:"Grafana"}),". Check out the ",(0,r.jsx)(n.a,{href:"monitoring",children:"Monitoring Juno"})," guide to get started."]})]}),"\n",(0,r.jsxs)(s,{children:[(0,r.jsx)("summary",{children:"Do node operators receive any rewards, or is participation solely to support the network?"}),(0,r.jsx)(n.p,{children:"Presently, running a node does not come with direct rewards; its primary purpose is contributing to the network's functionality and stability. However, operating a node provides valuable educational benefits and deepens your knowledge of the network's operation."})]}),"\n",(0,r.jsxs)(s,{children:[(0,r.jsx)("summary",{children:"How can I view Juno logs from Docker?"}),(0,r.jsx)(n.p,{children:"You can view logs from the Docker container using the following command:"}),(0,r.jsx)(n.pre,{children:(0,r.jsx)(n.code,{className:"language-bash",children:"docker logs -f juno\n"})})]}),"\n",(0,r.jsxs)(s,{children:[(0,r.jsx)("summary",{children:"How can I get real-time updates of new blocks?"}),(0,r.jsxs)(n.p,{children:["The ",(0,r.jsx)(n.a,{href:"websocket#subscribe-to-newly-created-blocks",children:"WebSocket"})," interface provides a ",(0,r.jsx)(n.code,{children:"juno_subscribeNewHeads"})," method that emits an event when new blocks are added to the blockchain."]})]}),"\n",(0,r.jsxs)(s,{children:[(0,r.jsx)("summary",{children:"Does Juno provide snapshots to sync with Starknet quickly?"}),(0,r.jsxs)(n.p,{children:["Yes, Juno provides snapshots for both the Starknet Mainnet and Sepolia networks. Check out the ",(0,r.jsx)(n.a,{href:"snapshots",children:"Database Snapshots"})," guide to get started."]})]}),"\n",(0,r.jsxs)(s,{children:[(0,r.jsx)("summary",{children:"How can I contribute to Juno?"}),(0,r.jsxs)(n.p,{children:["You can contribute to Juno by running a node, starring on GitHub, reporting bugs, and suggesting new features. Check out the ",(0,r.jsx)(n.a,{href:"/#contributions-and-partnerships",children:"Contributions and Partnerships"})," page for more information."]})]}),"\n",(0,r.jsxs)(s,{children:[(0,r.jsxs)("summary",{children:["I noticed a warning in my logs saying: ",(0,r.jsx)(n.strong,{children:'Failed\xa0storing\xa0Block {"err": "unsupported block version"}'}),". How should I proceed?"]}),(0,r.jsxs)(n.p,{children:["You can fix this problem by ",(0,r.jsx)(n.a,{href:"updating",children:"updating to the latest version"})," of Juno.\xa0Check for updates and install them to maintain compatibility with the latest block versions."]})]}),"\n",(0,r.jsxs)(s,{children:[(0,r.jsxs)("summary",{children:["After updating Juno, I receive ",(0,r.jsx)(n.strong,{children:"error while migrating DB."})," How should I proceed?"]}),(0,r.jsx)(n.p,{children:"This error suggests your database is corrupted, likely due to the node being interrupted during migration. This can occur if there are insufficient system resources, such as RAM, to finish the process. The only solution is to resynchronise the node from the beginning. To avoid this issue in the future, ensure your system has adequate resources and that the node remains uninterrupted during upgrades."})]}),"\n",(0,r.jsxs)(s,{children:[(0,r.jsxs)("summary",{children:["I receive ",(0,r.jsx)(n.strong,{children:"Error: unable to verify latest block hash; are the database and --network option compatible?"})," while running Juno. How should I proceed?"]}),(0,r.jsxs)(n.p,{children:["To resolve this issue, ensure that the ",(0,r.jsx)(n.code,{children:"eth-node"})," configuration aligns with the ",(0,r.jsx)(n.code,{children:"network"})," option for the Starknet network."]})]}),"\n",(0,r.jsxs)(s,{children:[(0,r.jsxs)("summary",{children:["I receive ",(0,r.jsx)(n.strong,{children:"process <PID> killed"})," and ",(0,r.jsx)(n.strong,{children:"./build/juno: invalid signature (code or signature have been modified)"})," while running the binary on macOS. How should I proceed?"]}),(0,r.jsx)(n.p,{children:"You need to re-sign the binary to resolve this issue using the following command:"}),(0,r.jsx)(n.pre,{children:(0,r.jsx)(n.code,{className:"language-bash",children:"codesign --sign - ./build/juno\n"})})]})]})}function h(e={}){const{wrapper:n}={...(0,o.R)(),...e.components};return n?(0,r.jsx)(n,{...e,children:(0,r.jsx)(u,{...e})}):u(e)}},8453:(e,n,s)=>{s.d(n,{R:()=>i,x:()=>a});var r=s(6540);const o={},t=r.createContext(o);function i(e){const n=r.useContext(t);return r.useMemo((function(){return"function"==typeof e?e(n):{...n,...e}}),[n,e])}function a(e){let n;return n=e.disableParentContext?"function"==typeof e.components?e.components(o):e.components||o:i(e.components),r.createElement(t.Provider,{value:n},e.children)}}}]);