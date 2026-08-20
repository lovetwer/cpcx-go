import { createRouter, createWebHashHistory } from 'vue-router'
import LoginView from '../views/LoginView.vue'
import AddView from '../views/AddView.vue'
import DrawView from '../views/DrawView.vue'
import BuyView from '../views/BuyView.vue'
import ProfileView from '../views/ProfileView.vue'
import ShareView from '../views/ShareView.vue'
import { isAuthed } from '../store/auth'

const routes = [
  { path: '/', redirect: '/add' },
  { path: '/login', component: LoginView, meta: { public: true } },
  { path: '/add', component: AddView, meta: { auth: true, title: '添加彩票' } },
  { path: '/draw', component: DrawView, meta: { auth: true, title: '开奖结果' } },
  { path: '/buy', component: BuyView, meta: { auth: true, title: '我的购彩' } },
  { path: '/profile', component: ProfileView, meta: { auth: true, title: '个人中心' } },
  { path: '/share', component: ShareView, meta: { public: true, title: '彩票分享' } },
  // 兼容旧路由
  { path: '/lottery', redirect: '/buy' },
  { path: '/recognize', redirect: '/add' },
  { path: '/wins', redirect: '/buy' },
]

const router = createRouter({
  // 用 hash 模式：Go 静态文件服务器无需做 SPA 回退，部署最稳
  history: createWebHashHistory(),
  routes,
  scrollBehavior: () => ({ top: 0 }),
})

router.beforeEach((to) => {
  if (to.meta.auth && !isAuthed()) return '/login'
  if (to.path === '/login' && isAuthed()) return '/add'
  return true
})

export default router
