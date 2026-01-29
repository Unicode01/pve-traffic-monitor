import { createRouter, createWebHistory } from 'vue-router'
import Layout from '@/views/Layout.vue'

const routes = [
  {
    path: '/',
    component: Layout,
    redirect: '/dashboard',
    children: [
      {
        path: 'dashboard',
        name: 'Dashboard',
        component: () => import('@/views/Dashboard.vue'),
        meta: { title: 'Dashboard' }
      },
      {
        path: 'charts',
        name: 'Charts',
        component: () => import('@/views/Charts.vue'),
        meta: { title: 'Charts' }
      },
      {
        path: 'vm/:id',
        name: 'VMDetail',
        component: () => import('@/views/VMDetail.vue'),
        meta: { title: 'VM Detail' }
      }
    ]
  }
]

const router = createRouter({
  history: createWebHistory('/'),
  routes
})

export default router
