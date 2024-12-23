"use strict";(self.webpackChunkjuno_docs=self.webpackChunkjuno_docs||[]).push([[4559],{7405:(e,n,t)=>{t.r(n),t.d(n,{assets:()=>u,contentTitle:()=>l,default:()=>b,frontMatter:()=>o,metadata:()=>i,toc:()=>d});var s=t(4848),r=t(8453),a=t(3859),c=t(9365);const o={title:"WebSocket Interface"},l="WebSocket Interface :globe_with_meridians:",i={id:"websocket",title:"WebSocket Interface",description:"Juno provides a WebSocket RPC interface that supports all of Starknet's JSON-RPC API endpoints and allows you to subscribe to newly created blocks.",source:"@site/docs/websocket.md",sourceDirName:".",slug:"/websocket",permalink:"/next/websocket",draft:!1,unlisted:!1,tags:[],version:"current",frontMatter:{title:"WebSocket Interface"},sidebar:"main",previous:{title:"JSON-RPC Interface",permalink:"/next/json-rpc"},next:{title:"Become a Staking Validator",permalink:"/next/staking-validator"}},u={},d=[{value:"Enable the WebSocket server",id:"enable-the-websocket-server",level:2},{value:"Making WebSocket requests",id:"making-websocket-requests",level:2},{value:"Subscribe to newly created blocks",id:"subscribe-to-newly-created-blocks",level:2},{value:"Unsubscribe from newly created blocks",id:"unsubscribe-from-newly-created-blocks",level:2},{value:"Testing the WebSocket connection",id:"testing-the-websocket-connection",level:2}];function h(e){const n={a:"a",code:"code",h1:"h1",h2:"h2",li:"li",p:"p",pre:"pre",ul:"ul",...(0,r.R)(),...e.components};return(0,s.jsxs)(s.Fragment,{children:[(0,s.jsxs)(n.h1,{id:"websocket-interface-globe_with_meridians",children:["WebSocket Interface ","\ud83c\udf10"]}),"\n",(0,s.jsxs)(n.p,{children:["Juno provides a WebSocket RPC interface that supports all of ",(0,s.jsx)(n.a,{href:"https://playground.open-rpc.org/?uiSchema%5BappBar%5D%5Bui:splitView%5D=false&schemaUrl=https://raw.githubusercontent.com/starkware-libs/starknet-specs/v0.7.0/api/starknet_api_openrpc.json&uiSchema%5BappBar%5D%5Bui:input%5D=false&uiSchema%5BappBar%5D%5Bui:darkMode%5D=true&uiSchema%5BappBar%5D%5Bui:examplesDropdown%5D=false",children:"Starknet's JSON-RPC API"})," endpoints and allows you to ",(0,s.jsx)(n.a,{href:"#subscribe-to-newly-created-blocks",children:"subscribe to newly created blocks"}),"."]}),"\n",(0,s.jsx)(n.h2,{id:"enable-the-websocket-server",children:"Enable the WebSocket server"}),"\n",(0,s.jsx)(n.p,{children:"To enable the WebSocket RPC server, use the following configuration options:"}),"\n",(0,s.jsxs)(n.ul,{children:["\n",(0,s.jsxs)(n.li,{children:[(0,s.jsx)(n.code,{children:"ws"}),": Enables the Websocket RPC server on the default port (disabled by default)."]}),"\n",(0,s.jsxs)(n.li,{children:[(0,s.jsx)(n.code,{children:"ws-host"}),": The interface on which the Websocket RPC server will listen for requests. If skipped, it defaults to ",(0,s.jsx)(n.code,{children:"localhost"}),"."]}),"\n",(0,s.jsxs)(n.li,{children:[(0,s.jsx)(n.code,{children:"ws-port"}),": The port on which the WebSocket server will listen for requests. If skipped, it defaults to ",(0,s.jsx)(n.code,{children:"6061"}),"."]}),"\n"]}),"\n",(0,s.jsx)(n.pre,{children:(0,s.jsx)(n.code,{className:"language-bash",children:"# Docker container\ndocker run -d \\\n  --name juno \\\n  -p 6061:6061 \\\n  nethermind/juno \\\n  --ws \\\n  --ws-port 6061 \\\n  --ws-host 0.0.0.0\n\n# Standalone binary\n./build/juno --ws --ws-port 6061 --ws-host 0.0.0.0\n"})}),"\n",(0,s.jsx)(n.h2,{id:"making-websocket-requests",children:"Making WebSocket requests"}),"\n",(0,s.jsxs)(n.p,{children:["You can use any of ",(0,s.jsx)(n.a,{href:"https://playground.open-rpc.org/?uiSchema%5BappBar%5D%5Bui:splitView%5D=false&schemaUrl=https://raw.githubusercontent.com/starkware-libs/starknet-specs/v0.7.0/api/starknet_api_openrpc.json&uiSchema%5BappBar%5D%5Bui:input%5D=false&uiSchema%5BappBar%5D%5Bui:darkMode%5D=true&uiSchema%5BappBar%5D%5Bui:examplesDropdown%5D=false",children:"Starknet's Node API Endpoints"})," with Juno. Check the availability of Juno with the ",(0,s.jsx)(n.code,{children:"juno_version"})," method:"]}),"\n","\n",(0,s.jsxs)(a.A,{children:[(0,s.jsx)(c.A,{value:"request",label:"Request",children:(0,s.jsx)(n.pre,{children:(0,s.jsx)(n.code,{className:"language-json",children:'{\n  "jsonrpc": "2.0",\n  "method": "juno_version",\n  "params": [],\n  "id": 1\n}\n'})})}),(0,s.jsx)(c.A,{value:"response",label:"Response",children:(0,s.jsx)(n.pre,{children:(0,s.jsx)(n.code,{className:"language-json",children:'{\n  "jsonrpc": "2.0",\n  "result": "v0.11.7",\n  "id": 1\n}\n'})})})]}),"\n",(0,s.jsxs)(n.p,{children:["Get the most recent accepted block hash and number with the ",(0,s.jsx)(n.code,{children:"starknet_blockHashAndNumber"})," method:"]}),"\n",(0,s.jsxs)(a.A,{children:[(0,s.jsx)(c.A,{value:"request",label:"Request",children:(0,s.jsx)(n.pre,{children:(0,s.jsx)(n.code,{className:"language-json",children:'{\n  "jsonrpc": "2.0",\n  "method": "starknet_blockHashAndNumber",\n  "params": [],\n  "id": 1\n}\n'})})}),(0,s.jsx)(c.A,{value:"response",label:"Response",children:(0,s.jsx)(n.pre,{children:(0,s.jsx)(n.code,{className:"language-json",children:'{\n  "jsonrpc": "2.0",\n  "result": {\n    "block_hash": "0x637ae4d7468bb603c2f16ba7f9118d58c7d7c98a8210260372e83e7c9df443a",\n    "block_number": 640827\n  },\n  "id": 1\n}\n'})})})]}),"\n",(0,s.jsx)(n.h2,{id:"subscribe-to-newly-created-blocks",children:"Subscribe to newly created blocks"}),"\n",(0,s.jsxs)(n.p,{children:["The WebSocket server provides a ",(0,s.jsx)(n.code,{children:"starknet_subscribeNewHeads"})," method that emits an event when new blocks are added to the blockchain:"]}),"\n",(0,s.jsxs)(a.A,{children:[(0,s.jsx)(c.A,{value:"request",label:"Request",children:(0,s.jsx)(n.pre,{children:(0,s.jsx)(n.code,{className:"language-json",children:'{\n  "jsonrpc": "2.0",\n  "method": "starknet_subscribeNewHeads",\n  "id": 1\n}\n'})})}),(0,s.jsx)(c.A,{value:"response",label:"Response",children:(0,s.jsx)(n.pre,{children:(0,s.jsx)(n.code,{className:"language-json",children:'{\n  "jsonrpc": "2.0",\n  "result": 16570962336122680234,\n  "id": 1\n}\n'})})})]}),"\n",(0,s.jsx)(n.p,{children:"When a new block is added, you will receive a message like this:"}),"\n",(0,s.jsx)(n.pre,{children:(0,s.jsx)(n.code,{className:"language-json",children:'{\n  "jsonrpc": "2.0",\n  "method": "starknet_subscriptionNewHeads",\n  "params": {\n    "result": {\n      "block_hash": "0x840660a07a17ae6a55d39fb6d366698ecda11e02280ca3e9ca4b4f1bad741c",\n      "parent_hash": "0x529ca67a127e4f40f3ae637fc54c7a56c853b2e085011c64364911af74c9a5c",\n      "block_number": 65644,\n      "new_root": "0x4e88ddf34b52091611b34d72849e230d329902888eb57c8e3c1b9cc180a426c",\n      "timestamp": 1715451809,\n      "sequencer_address": "0x1176a1bd84444c89232ec27754698e5d2e7e1a7f1539f12027f28b23ec9f3d8",\n      "l1_gas_price": {\n        "price_in_fri": "0x3727bcc63f1",\n        "price_in_wei": "0x5f438c77"\n      },\n      "l1_data_gas_price": {\n        "price_in_fri": "0x230e40e8866c6e",\n        "price_in_wei": "0x3c8c4a9ea51"\n      },\n      "l1_da_mode": "BLOB",\n      "starknet_version": "0.13.1.1"\n    },\n    "subscription_id": 16570962336122680234\n  }\n}\n'})}),"\n",(0,s.jsx)(n.h2,{id:"unsubscribe-from-newly-created-blocks",children:"Unsubscribe from newly created blocks"}),"\n",(0,s.jsxs)(n.p,{children:["Use the ",(0,s.jsx)(n.code,{children:"juno_unsubscribe"})," method with the ",(0,s.jsx)(n.code,{children:"result"})," value from the subscription response or the ",(0,s.jsx)(n.code,{children:"subscription"})," field from any new block event to stop receiving updates for new blocks:"]}),"\n",(0,s.jsxs)(a.A,{children:[(0,s.jsx)(c.A,{value:"request",label:"Request",children:(0,s.jsx)(n.pre,{children:(0,s.jsx)(n.code,{className:"language-json",children:'{\n  "jsonrpc": "2.0",\n  "method": "juno_unsubscribe",\n  "params": {\n    "id": 16570962336122680234\n  },\n  "id": 1\n}\n'})})}),(0,s.jsx)(c.A,{value:"response",label:"Response",children:(0,s.jsx)(n.pre,{children:(0,s.jsx)(n.code,{className:"language-json",children:'{\n  "jsonrpc": "2.0",\n  "result": true,\n  "id": 1\n}\n'})})})]}),"\n",(0,s.jsx)(n.h2,{id:"testing-the-websocket-connection",children:"Testing the WebSocket connection"}),"\n",(0,s.jsxs)(n.p,{children:["You can test your WebSocket connection using tools like ",(0,s.jsx)(n.a,{href:"https://github.com/websockets/wscat",children:"wscat"})," or ",(0,s.jsx)(n.a,{href:"https://github.com/vi/websocat",children:"websocat"}),":"]}),"\n",(0,s.jsx)(n.pre,{children:(0,s.jsx)(n.code,{className:"language-bash",children:'# wscat\n$ wscat -c ws://localhost:6061\n    > {"jsonrpc": "2.0", "method": "juno_version", "id": 1}\n    < {"jsonrpc": "2.0", "result": "v0.11.7", "id": 1}\n\n# websocat\n$ websocat -v ws://localhost:6061\n    [INFO  websocat::lints] Auto-inserting the line mode\n    [INFO  websocat::stdio_threaded_peer] get_stdio_peer (threaded)\n    [INFO  websocat::ws_client_peer] get_ws_client_peer\n    [INFO  websocat::ws_client_peer] Connected to ws\n    {"jsonrpc": "2.0", "method": "juno_version", "id": 1}\n    {"jsonrpc": "2.0", "result": "v0.11.7", "id": 1}\n'})})]})}function b(e={}){const{wrapper:n}={...(0,r.R)(),...e.components};return n?(0,s.jsx)(n,{...e,children:(0,s.jsx)(h,{...e})}):h(e)}},9365:(e,n,t)=>{t.d(n,{A:()=>c});t(6540);var s=t(4164);const r={tabItem:"tabItem_Ymn6"};var a=t(4848);function c(e){let{children:n,hidden:t,className:c}=e;return(0,a.jsx)("div",{role:"tabpanel",className:(0,s.A)(r.tabItem,c),hidden:t,children:n})}},3859:(e,n,t)=>{t.d(n,{A:()=>g});var s=t(6540),r=t(4164),a=t(6641),c=t(6347),o=t(205),l=t(8874),i=t(4035),u=t(2993);function d(e){return s.Children.toArray(e).filter((e=>"\n"!==e)).map((e=>{if(!e||(0,s.isValidElement)(e)&&function(e){const{props:n}=e;return!!n&&"object"==typeof n&&"value"in n}(e))return e;throw new Error(`Docusaurus error: Bad <Tabs> child <${"string"==typeof e.type?e.type:e.type.name}>: all children of the <Tabs> component should be <TabItem>, and every <TabItem> should have a unique "value" prop.`)}))?.filter(Boolean)??[]}function h(e){const{values:n,children:t}=e;return(0,s.useMemo)((()=>{const e=n??function(e){return d(e).map((e=>{let{props:{value:n,label:t,attributes:s,default:r}}=e;return{value:n,label:t,attributes:s,default:r}}))}(t);return function(e){const n=(0,i.X)(e,((e,n)=>e.value===n.value));if(n.length>0)throw new Error(`Docusaurus error: Duplicate values "${n.map((e=>e.value)).join(", ")}" found in <Tabs>. Every value needs to be unique.`)}(e),e}),[n,t])}function b(e){let{value:n,tabValues:t}=e;return t.some((e=>e.value===n))}function p(e){let{queryString:n=!1,groupId:t}=e;const r=(0,c.W6)(),a=function(e){let{queryString:n=!1,groupId:t}=e;if("string"==typeof n)return n;if(!1===n)return null;if(!0===n&&!t)throw new Error('Docusaurus error: The <Tabs> component groupId prop is required if queryString=true, because this value is used as the search param name. You can also provide an explicit value such as queryString="my-search-param".');return t??null}({queryString:n,groupId:t});return[(0,l.aZ)(a),(0,s.useCallback)((e=>{if(!a)return;const n=new URLSearchParams(r.location.search);n.set(a,e),r.replace({...r.location,search:n.toString()})}),[a,r])]}function f(e){const{defaultValue:n,queryString:t=!1,groupId:r}=e,a=h(e),[c,l]=(0,s.useState)((()=>function(e){let{defaultValue:n,tabValues:t}=e;if(0===t.length)throw new Error("Docusaurus error: the <Tabs> component requires at least one <TabItem> children component");if(n){if(!b({value:n,tabValues:t}))throw new Error(`Docusaurus error: The <Tabs> has a defaultValue "${n}" but none of its children has the corresponding value. Available values are: ${t.map((e=>e.value)).join(", ")}. If you intend to show no default tab, use defaultValue={null} instead.`);return n}const s=t.find((e=>e.default))??t[0];if(!s)throw new Error("Unexpected error: 0 tabValues");return s.value}({defaultValue:n,tabValues:a}))),[i,d]=p({queryString:t,groupId:r}),[f,m]=function(e){let{groupId:n}=e;const t=function(e){return e?`docusaurus.tab.${e}`:null}(n),[r,a]=(0,u.Dv)(t);return[r,(0,s.useCallback)((e=>{t&&a.set(e)}),[t,a])]}({groupId:r}),j=(()=>{const e=i??f;return b({value:e,tabValues:a})?e:null})();(0,o.A)((()=>{j&&l(j)}),[j]);return{selectedValue:c,selectValue:(0,s.useCallback)((e=>{if(!b({value:e,tabValues:a}))throw new Error(`Can't select invalid tab value=${e}`);l(e),d(e),m(e)}),[d,m,a]),tabValues:a}}var m=t(2303);const j={tabList:"tabList__CuJ",tabItem:"tabItem_LNqP"};var w=t(4848);function x(e){let{className:n,block:t,selectedValue:s,selectValue:c,tabValues:o}=e;const l=[],{blockElementScrollPositionUntilNextRender:i}=(0,a.a_)(),u=e=>{const n=e.currentTarget,t=l.indexOf(n),r=o[t].value;r!==s&&(i(n),c(r))},d=e=>{let n=null;switch(e.key){case"Enter":u(e);break;case"ArrowRight":{const t=l.indexOf(e.currentTarget)+1;n=l[t]??l[0];break}case"ArrowLeft":{const t=l.indexOf(e.currentTarget)-1;n=l[t]??l[l.length-1];break}}n?.focus()};return(0,w.jsx)("ul",{role:"tablist","aria-orientation":"horizontal",className:(0,r.A)("tabs",{"tabs--block":t},n),children:o.map((e=>{let{value:n,label:t,attributes:a}=e;return(0,w.jsx)("li",{role:"tab",tabIndex:s===n?0:-1,"aria-selected":s===n,ref:e=>l.push(e),onKeyDown:d,onClick:u,...a,className:(0,r.A)("tabs__item",j.tabItem,a?.className,{"tabs__item--active":s===n}),children:t??n},n)}))})}function k(e){let{lazy:n,children:t,selectedValue:r}=e;const a=(Array.isArray(t)?t:[t]).filter(Boolean);if(n){const e=a.find((e=>e.props.value===r));return e?(0,s.cloneElement)(e,{className:"margin-top--md"}):null}return(0,w.jsx)("div",{className:"margin-top--md",children:a.map(((e,n)=>(0,s.cloneElement)(e,{key:n,hidden:e.props.value!==r})))})}function v(e){const n=f(e);return(0,w.jsxs)("div",{className:(0,r.A)("tabs-container",j.tabList),children:[(0,w.jsx)(x,{...e,...n}),(0,w.jsx)(k,{...e,...n})]})}function g(e){const n=(0,m.A)();return(0,w.jsx)(v,{...e,children:d(e.children)},String(n))}},8453:(e,n,t)=>{t.d(n,{R:()=>c,x:()=>o});var s=t(6540);const r={},a=s.createContext(r);function c(e){const n=s.useContext(a);return s.useMemo((function(){return"function"==typeof e?e(n):{...n,...e}}),[n,e])}function o(e){let n;return n=e.disableParentContext?"function"==typeof e.components?e.components(r):e.components||r:c(e.components),s.createElement(a.Provider,{value:n},e.children)}}}]);