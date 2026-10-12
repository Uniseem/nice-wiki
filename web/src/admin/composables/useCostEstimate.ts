import { ref } from 'vue'
export function useCostEstimate<T>() { const estimate=ref<T>();const open=ref(false);const busy=ref(false);async function prepare(load:()=>Promise<T>){busy.value=true;try{estimate.value=await load();open.value=true}finally{busy.value=false}}function close(){open.value=false}return{estimate,open,busy,prepare,close} }
