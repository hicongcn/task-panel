import request from './request'

export const migrateApi = {
  export: () => request.get('/migrate/export'),
  import: (data: any) => request.post('/migrate/import', data, { timeout: 60000 }),
}
