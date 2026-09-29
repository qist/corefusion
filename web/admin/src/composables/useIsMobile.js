import { ref, onMounted, onBeforeUnmount } from 'vue'

// 移动端判定：≤768px 视为手机。全站共用，避免各页面重复监听 resize。
const isMobile = ref(typeof window !== 'undefined' ? window.innerWidth <= 768 : false)
let listenerCount = 0
const onResize = () => { isMobile.value = window.innerWidth <= 768 }

export function useIsMobile() {
  onMounted(() => {
    if (listenerCount === 0) window.addEventListener('resize', onResize)
    listenerCount++
    onResize()
  })
  onBeforeUnmount(() => {
    listenerCount--
    if (listenerCount === 0) window.removeEventListener('resize', onResize)
  })
  return { isMobile }
}
