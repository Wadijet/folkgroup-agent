# Code Examples

Các ví dụ code thực tế để implement frontend application tích hợp với FolkForm Auth Backend API.

## 📋 Mục Lục

- [React Examples](#react-examples)
- [Vue Examples](#vue-examples)
- [Angular Examples](#angular-examples)
- [Vanilla JavaScript Examples](#vanilla-javascript-examples)

---

## React Examples

### 1. Login Component

```typescript
import React, { useState } from 'react';
import { useAuth } from '../contexts/AuthContext';
import { firebaseService } from '../services/firebaseService';
import { handleApiError } from '../utils/errorHandler';

export function LoginPage() {
  const { login } = useAuth();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError('');

    try {
      await firebaseService.loginWithEmail(email, password);
      // Login successful, user will be set in context
    } catch (err) {
      setError(handleApiError(err));
    } finally {
      setLoading(false);
    }
  };

  const handleGoogleLogin = async () => {
    setLoading(true);
    setError('');

    try {
      await firebaseService.loginWithGoogle();
    } catch (err) {
      setError(handleApiError(err));
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="login-page">
      <form onSubmit={handleSubmit}>
        <input
          type="email"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          placeholder="Email"
          required
        />
        <input
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          placeholder="Password"
          required
        />
        <button type="submit" disabled={loading}>
          {loading ? 'Đang đăng nhập...' : 'Đăng nhập'}
        </button>
      </form>

      <button onClick={handleGoogleLogin} disabled={loading}>
        Đăng nhập với Google
      </button>

      {error && <div className="error">{error}</div>}
    </div>
  );
}
```

### 2. User List Component với Pagination

```typescript
import React, { useState, useEffect } from 'react';
import { userService } from '../services/userService';
import type { User, PaginatedResponse } from '../types';
import { handleApiError } from '../utils/errorHandler';

export function UserList() {
  const [users, setUsers] = useState<User[]>([]);
  const [pagination, setPagination] = useState({
    page: 1,
    limit: 10,
    itemCount: 0,
  });
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    loadUsers();
  }, [pagination.page]);

  const loadUsers = async () => {
    setLoading(true);
    setError('');

    try {
      const response: PaginatedResponse<User> = await userService.findWithPagination(
        pagination.page,
        pagination.limit
      );
      setUsers(response.items);
      setPagination((prev) => ({
        ...prev,
        itemCount: response.itemCount,
      }));
    } catch (err) {
      setError(handleApiError(err));
    } finally {
      setLoading(false);
    }
  };

  const handlePageChange = (newPage: number) => {
    setPagination((prev) => ({ ...prev, page: newPage }));
  };

  if (loading) return <div>Đang tải...</div>;
  if (error) return <div className="error">{error}</div>;

  return (
    <div>
      <h2>Danh sách người dùng</h2>
      <table>
        <thead>
          <tr>
            <th>ID</th>
            <th>Tên</th>
            <th>Email</th>
            <th>Ngày tạo</th>
          </tr>
        </thead>
        <tbody>
          {users.map((user) => (
            <tr key={user.id}>
              <td>{user.id}</td>
              <td>{user.name}</td>
              <td>{user.email}</td>
              <td>{new Date(user.createdAt * 1000).toLocaleDateString()}</td>
            </tr>
          ))}
        </tbody>
      </table>

      <div className="pagination">
        <button
          onClick={() => handlePageChange(pagination.page - 1)}
          disabled={pagination.page === 1}
        >
          Trước
        </button>
        <span>
          Trang {pagination.page} / {Math.ceil(pagination.itemCount / pagination.limit)}
        </span>
        <button
          onClick={() => handlePageChange(pagination.page + 1)}
          disabled={pagination.page * pagination.limit >= pagination.itemCount}
        >
          Sau
        </button>
      </div>
    </div>
  );
}
```

### 3. Profile Component

```typescript
import React, { useState, useEffect } from 'react';
import { useAuth } from '../contexts/AuthContext';
import { handleApiError } from '../utils/errorHandler';

export function ProfilePage() {
  const { user, updateProfile } = useAuth();
  const [name, setName] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');

  useEffect(() => {
    if (user) {
      setName(user.name);
    }
  }, [user]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError('');
    setSuccess('');

    try {
      await updateProfile(name);
      setSuccess('Cập nhật profile thành công!');
    } catch (err) {
      setError(handleApiError(err));
    } finally {
      setLoading(false);
    }
  };

  if (!user) {
    return <div>Đang tải...</div>;
  }

  return (
    <div className="profile-page">
      <h2>Thông tin cá nhân</h2>
      <form onSubmit={handleSubmit}>
        <div>
          <label>Email:</label>
          <input type="email" value={user.email || ''} disabled />
          <small>Email được quản lý bởi Firebase</small>
        </div>
        <div>
          <label>Số điện thoại:</label>
          <input type="tel" value={user.phone || ''} disabled />
          <small>Số điện thoại được quản lý bởi Firebase</small>
        </div>
        <div>
          <label>Tên:</label>
          <input
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            required
          />
        </div>
        <button type="submit" disabled={loading}>
          {loading ? 'Đang cập nhật...' : 'Cập nhật'}
        </button>
      </form>

      {error && <div className="error">{error}</div>}
      {success && <div className="success">{success}</div>}
    </div>
  );
}
```

### 4. Protected Route Component

```typescript
import React from 'react';
import { Navigate } from 'react-router-dom';
import { useAuth } from '../contexts/AuthContext';

interface ProtectedRouteProps {
  children: React.ReactNode;
}

export function ProtectedRoute({ children }: ProtectedRouteProps) {
  const { user, loading } = useAuth();

  if (loading) {
    return <div>Đang kiểm tra...</div>;
  }

  if (!user) {
    return <Navigate to="/login" replace />;
  }

  return <>{children}</>;
}
```

---

## Vue Examples

### 1. Login Component (Vue 3 Composition API)

```vue
<template>
  <div class="login-page">
    <form @submit.prevent="handleSubmit">
      <input
        v-model="email"
        type="email"
        placeholder="Email"
        required
      />
      <input
        v-model="password"
        type="password"
        placeholder="Password"
        required
      />
      <button type="submit" :disabled="loading">
        {{ loading ? 'Đang đăng nhập...' : 'Đăng nhập' }}
      </button>
    </form>

    <button @click="handleGoogleLogin" :disabled="loading">
      Đăng nhập với Google
    </button>

    <div v-if="error" class="error">{{ error }}</div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import { useRouter } from 'vue-router';
import { firebaseService } from '../services/firebaseService';
import { handleApiError } from '../utils/errorHandler';

const router = useRouter();
const email = ref('');
const password = ref('');
const loading = ref(false);
const error = ref('');

const handleSubmit = async () => {
  loading.value = true;
  error.value = '';

  try {
    await firebaseService.loginWithEmail(email.value, password.value);
    router.push('/dashboard');
  } catch (err) {
    error.value = handleApiError(err);
  } finally {
    loading.value = false;
  }
};

const handleGoogleLogin = async () => {
  loading.value = true;
  error.value = '';

  try {
    await firebaseService.loginWithGoogle();
    router.push('/dashboard');
  } catch (err) {
    error.value = handleApiError(err);
  } finally {
    loading.value = false;
  }
};
</script>
```

### 2. User List Component (Vue 3)

```vue
<template>
  <div>
    <h2>Danh sách người dùng</h2>
    <div v-if="loading">Đang tải...</div>
    <div v-else-if="error" class="error">{{ error }}</div>
    <table v-else>
      <thead>
        <tr>
          <th>ID</th>
          <th>Tên</th>
          <th>Email</th>
          <th>Ngày tạo</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="user in users" :key="user.id">
          <td>{{ user.id }}</td>
          <td>{{ user.name }}</td>
          <td>{{ user.email }}</td>
          <td>{{ formatDate(user.createdAt) }}</td>
        </tr>
      </tbody>
    </table>

    <div class="pagination">
      <button
        @click="handlePageChange(pagination.page - 1)"
        :disabled="pagination.page === 1"
      >
        Trước
      </button>
      <span>
        Trang {{ pagination.page }} / {{ totalPages }}
      </span>
      <button
        @click="handlePageChange(pagination.page + 1)"
        :disabled="pagination.page * pagination.limit >= pagination.itemCount"
      >
        Sau
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue';
import { userService } from '../services/userService';
import type { User, PaginatedResponse } from '../types';
import { handleApiError } from '../utils/errorHandler';

const users = ref<User[]>([]);
const pagination = ref({
  page: 1,
  limit: 10,
  itemCount: 0,
});
const loading = ref(false);
const error = ref('');

const totalPages = computed(() =>
  Math.ceil(pagination.value.itemCount / pagination.value.limit)
);

const loadUsers = async () => {
  loading.value = true;
  error.value = '';

  try {
    const response: PaginatedResponse<User> = await userService.findWithPagination(
      pagination.value.page,
      pagination.value.limit
    );
    users.value = response.items;
    pagination.value.itemCount = response.itemCount;
  } catch (err) {
    error.value = handleApiError(err);
  } finally {
    loading.value = false;
  }
};

const handlePageChange = (newPage: number) => {
  pagination.value.page = newPage;
};

const formatDate = (timestamp: number) => {
  return new Date(timestamp * 1000).toLocaleDateString();
};

onMounted(() => {
  loadUsers();
});

watch(() => pagination.value.page, () => {
  loadUsers();
});
</script>
```

---

## Angular Examples

### 1. Auth Service (Angular)

```typescript
import { Injectable } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Observable, BehaviorSubject } from 'rxjs';
import { tap } from 'rxjs/operators';
import { environment } from '../environments/environment';
import type { User } from '../types';

@Injectable({
  providedIn: 'root'
})
export class AuthService {
  private apiUrl = `${environment.apiUrl}/auth`;
  private currentUserSubject = new BehaviorSubject<User | null>(null);
  public currentUser$ = this.currentUserSubject.asObservable();

  constructor(private http: HttpClient) {
    // Load user from localStorage on init
    const token = localStorage.getItem('auth_token');
    if (token) {
      this.getProfile().subscribe();
    }
  }

  loginWithFirebase(idToken: string, hwid: string): Observable<User> {
    return this.http.post<User>(`${this.apiUrl}/login/firebase`, {
      idToken,
      hwid
    }).pipe(
      tap(user => {
        if (user.token) {
          localStorage.setItem('auth_token', user.token);
          this.currentUserSubject.next(user);
        }
      })
    );
  }

  logout(): Observable<void> {
    const hwid = this.getHWID();
    return this.http.post<void>(`${this.apiUrl}/logout`, { hwid }).pipe(
      tap(() => {
        localStorage.removeItem('auth_token');
        this.currentUserSubject.next(null);
      })
    );
  }

  getProfile(): Observable<User> {
    return this.http.get<User>(`${this.apiUrl}/profile`).pipe(
      tap(user => this.currentUserSubject.next(user))
    );
  }

  updateProfile(name: string): Observable<User> {
    return this.http.put<User>(`${this.apiUrl}/profile`, { name }).pipe(
      tap(user => this.currentUserSubject.next(user))
    );
  }

  private getHWID(): string {
    let hwid = localStorage.getItem('hwid');
    if (!hwid) {
      hwid = `hwid_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;
      localStorage.setItem('hwid', hwid);
    }
    return hwid;
  }
}
```

### 2. Auth Guard (Angular)

```typescript
import { Injectable } from '@angular/core';
import { CanActivate, Router } from '@angular/router';
import { AuthService } from './auth.service';
import { map, take } from 'rxjs/operators';

@Injectable({
  providedIn: 'root'
})
export class AuthGuard implements CanActivate {
  constructor(
    private authService: AuthService,
    private router: Router
  ) {}

  canActivate(): Observable<boolean> {
    return this.authService.currentUser$.pipe(
      take(1),
      map(user => {
        if (user) {
          return true;
        } else {
          this.router.navigate(['/login']);
          return false;
        }
      })
    );
  }
}
```

---

## Vanilla JavaScript Examples

### 1. Simple API Client

```javascript
class ApiClient {
  constructor(baseUrl = 'http://localhost:8080/api/v1') {
    this.baseUrl = baseUrl;
    this.token = localStorage.getItem('auth_token');
    this.hwid = this.getOrCreateHWID();
  }

  getOrCreateHWID() {
    let hwid = localStorage.getItem('hwid');
    if (!hwid) {
      hwid = `hwid_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;
      localStorage.setItem('hwid', hwid);
    }
    return hwid;
  }

  setToken(token) {
    this.token = token;
    localStorage.setItem('auth_token', token);
  }

  async request(endpoint, options = {}) {
    const headers = {
      'Content-Type': 'application/json',
      ...options.headers,
    };

    if (this.token) {
      headers['Authorization'] = `Bearer ${this.token}`;
    }

    const response = await fetch(`${this.baseUrl}${endpoint}`, {
      ...options,
      headers,
    });

    const data = await response.json();

    if (!response.ok || data.status === 'error') {
      throw new Error(data.message || 'API Error');
    }

    return data;
  }

  async loginWithFirebase(idToken) {
    const response = await this.request('/auth/login/firebase', {
      method: 'POST',
      body: JSON.stringify({
        idToken,
        hwid: this.hwid,
      }),
    });

    if (response.data.token) {
      this.setToken(response.data.token);
    }

    return response.data;
  }

  async logout() {
    await this.request('/auth/logout', {
      method: 'POST',
      body: JSON.stringify({ hwid: this.hwid }),
    });
    this.token = null;
    localStorage.removeItem('auth_token');
  }
}

// Usage
const apiClient = new ApiClient();

async function login() {
  try {
    const user = await apiClient.loginWithFirebase(firebaseIdToken);
    console.log('Logged in:', user);
  } catch (error) {
    console.error('Login failed:', error);
  }
}
```

---

**Cập nhật lần cuối**: 2025-12-10

