import client from './client'

export const runtimeOverview = () =>
  client({ url: '/runtime/overview', method: 'get' })

export const listSnapshots = (params) =>
  client({ url: '/runtime/snapshots', method: 'get', params })
export const createSnapshot = (data) =>
  client({ url: '/runtime/snapshots', method: 'post', data })
export const getSnapshot = (id) =>
  client({ url: `/runtime/snapshots/${id}`, method: 'get' })
export const deleteSnapshot = (id) =>
  client({ url: `/runtime/snapshots/${id}`, method: 'delete' })

export const dryRun = () =>
  client({ url: '/runtime/dry-run', method: 'post' })

export const applyRuntime = (data) =>
  client({ url: '/runtime/apply', method: 'post', data })
export const rollbackRuntime = (data) =>
  client({ url: '/runtime/rollback', method: 'post', data })
export const disableProxy = (data) =>
  client({ url: '/runtime/disable-proxy', method: 'post', data })

export const listJobs = (params) =>
  client({ url: '/runtime/jobs', method: 'get', params })
export const getJob = (id) =>
  client({ url: `/runtime/jobs/${id}`, method: 'get' })
export const retryJob = (id) =>
  client({ url: `/runtime/jobs/${id}/retry`, method: 'post' })
export const cancelJob = (id) =>
  client({ url: `/runtime/jobs/${id}/cancel`, method: 'post' })
