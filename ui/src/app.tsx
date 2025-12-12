import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { Toaster } from '@/components/ui/sonner';
import { FlowListPage } from '@/pages/flow-list-page';
import { FlowEditorPage } from '@/pages/flow-editor-page';
import { LoginPage } from '@/pages/login-page';
import { Layout } from '@/components/layout';

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 5000,
      refetchOnWindowFocus: false,
    },
  },
});

function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route path="/" element={<Layout />}>
            <Route index element={<Navigate to="/flows" replace />} />
            <Route path="flows" element={<FlowListPage />} />
            <Route path="flows/:id" element={<FlowEditorPage />} />
          </Route>
        </Routes>
      </BrowserRouter>
      <Toaster position="bottom-right" />
    </QueryClientProvider>
  );
}

export default App;
